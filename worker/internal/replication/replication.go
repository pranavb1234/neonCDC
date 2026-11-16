package replication

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
)

const outputPlugin = "pgoutput"

type Config struct {
	SourceDBURL     string
	SlotName        string
	PublicationName string
	Tables          []Table
}

type Table struct {
	Schema string
	Name   string
}

type EventPublisher interface {
	Publish(context.Context, []Event) error
}

func Run(ctx context.Context, db *pgx.Conn, cfg Config, publisher EventPublisher, logger *slog.Logger) error {
	if err := verifyPrerequisites(ctx, db); err != nil {
		return err
	}
	if err := ensurePublication(ctx, db, cfg.PublicationName, cfg.Tables); err != nil {
		return err
	}

	replicationURL, err := replicationConnectionURL(cfg.SourceDBURL)
	if err != nil {
		return errors.New("could not prepare the replication connection")
	}

	conn, err := pgconn.Connect(ctx, replicationURL)
	if err != nil {
		return errors.New("could not open PostgreSQL replication connection; use Neon's direct endpoint and verify network access")
	}
	defer conn.Close(context.Background())

	startLSN, err := ensureSlot(ctx, conn, cfg.SlotName)
	if err != nil {
		return err
	}

	if err := pglogrepl.StartReplication(ctx, conn, cfg.SlotName, startLSN, pglogrepl.StartReplicationOptions{
		Mode: pglogrepl.LogicalReplication,
		PluginArgs: []string{
			"proto_version '1'",
			fmt.Sprintf("publication_names '%s'", cfg.PublicationName),
		},
	}); err != nil {
		return replicationError("could not start logical replication", err)
	}

	logger.Info("logical replication started", "slot", cfg.SlotName, "publication", cfg.PublicationName)
	return readMessages(ctx, conn, publisher, logger)
}

func verifyPrerequisites(ctx context.Context, db *pgx.Conn) error {
	var walLevel string
	if err := db.QueryRow(ctx, "SHOW wal_level").Scan(&walLevel); err != nil {
		return errors.New("could not check wal_level; enable logical replication in Neon project settings")
	}
	if walLevel != "logical" {
		return errors.New("logical replication is disabled; enable it in Neon project settings and wait for the compute restart")
	}

	var canReplicate bool
	if err := db.QueryRow(ctx, "SELECT rolreplication FROM pg_roles WHERE rolname = current_user").Scan(&canReplicate); err != nil {
		return errors.New("could not verify the database role's replication permission")
	}
	if !canReplicate {
		return errors.New("database role lacks REPLICATION permission; use a role created through Neon Console, CLI, or API")
	}

	return nil
}

func ensurePublication(ctx context.Context, db *pgx.Conn, publicationName string, tables []Table) error {
	if len(tables) == 0 {
		return errors.New("at least one table must be selected for the CDC publication")
	}
	var exists bool
	if err := db.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_publication WHERE pubname = $1)", publicationName).Scan(&exists); err != nil {
		return errors.New("could not check the CDC publication")
	}
	if exists {
		return nil
	}

	quotedTables := make([]string, 0, len(tables))
	for _, table := range tables {
		quotedTables = append(quotedTables, pgx.Identifier{table.Schema, table.Name}.Sanitize())
	}
	statement := fmt.Sprintf("CREATE PUBLICATION %s FOR TABLE %s", pgx.Identifier{publicationName}.Sanitize(), strings.Join(quotedTables, ", "))
	if _, err := db.Exec(ctx, statement); err != nil {
		return errors.New("could not create the CDC publication; use the table owner and verify the selected tables")
	}
	return nil
}

func ensureSlot(ctx context.Context, conn *pgconn.PgConn, slotName string) (pglogrepl.LSN, error) {
	created, err := pglogrepl.CreateReplicationSlot(ctx, conn, slotName, outputPlugin, pglogrepl.CreateReplicationSlotOptions{
		Mode: pglogrepl.LogicalReplication,
	})
	if err == nil {
		startLSN, parseErr := pglogrepl.ParseLSN(created.ConsistentPoint)
		if parseErr != nil {
			return 0, errors.New("could not read the new replication slot position")
		}
		return startLSN, nil
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42710" {
		return 0, nil
	}
	return 0, replicationError("could not create replication slot; enable logical replication and use a Neon role with REPLICATION permission", err)
}

func readMessages(ctx context.Context, conn *pgconn.PgConn, publisher EventPublisher, logger *slog.Logger) error {
	decoder := newEventDecoder()
	for {
		message, err := conn.ReceiveMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return errors.New("logical replication connection closed unexpectedly")
		}

		copyData, ok := message.(*pgproto3.CopyData)
		if !ok || len(copyData.Data) == 0 {
			continue
		}

		switch copyData.Data[0] {
		case pglogrepl.PrimaryKeepaliveMessageByteID:
			keepalive, err := pglogrepl.ParsePrimaryKeepaliveMessage(copyData.Data[1:])
			if err != nil {
				return errors.New("could not parse PostgreSQL keepalive message")
			}
			logger.Debug("replication keepalive", "reply_requested", keepalive.ReplyRequested)
			if keepalive.ReplyRequested {
				if err := pglogrepl.SendStandbyStatusUpdate(ctx, conn, pglogrepl.StandbyStatusUpdate{}); err != nil {
					return errors.New("could not reply to PostgreSQL keepalive")
				}
			}
		case pglogrepl.XLogDataByteID:
			xlogData, err := pglogrepl.ParseXLogData(copyData.Data[1:])
			if err != nil {
				return errors.New("could not parse PostgreSQL XLogData message")
			}
			logicalMessage, err := pglogrepl.Parse(xlogData.WALData)
			if err != nil {
				return errors.New("could not parse pgoutput message")
			}
			logger.Info("replication message", "type", logicalMessage.Type().String(), "wal_start", xlogData.WALStart.String())
			events, err := decoder.Process(logicalMessage)
			if err != nil {
				return err
			}
			for _, event := range events {
				logger.Info("cdc event", "event", event)
			}
			if commit, ok := logicalMessage.(*pglogrepl.CommitMessage); ok {
				if err := publisher.Publish(ctx, events); err != nil {
					return errors.New("could not publish committed events to Redis; PostgreSQL LSN was not acknowledged")
				}
				if err := acknowledgeCommit(ctx, conn, commit.TransactionEndLSN); err != nil {
					return errors.New("could not acknowledge PostgreSQL LSN after Redis publish")
				}
				logger.Info("transaction published and acknowledged", "event_count", len(events), "commit_lsn", commit.CommitLSN.String())
			}
		}
	}
}

func acknowledgeCommit(ctx context.Context, conn *pgconn.PgConn, lsn pglogrepl.LSN) error {
	return pglogrepl.SendStandbyStatusUpdate(ctx, conn, pglogrepl.StandbyStatusUpdate{
		WALWritePosition: lsn,
		WALFlushPosition: lsn,
		WALApplyPosition: lsn,
	})
}

func replicationConnectionURL(sourceDBURL string) (string, error) {
	connectionURL, err := url.Parse(sourceDBURL)
	if err != nil {
		return "", err
	}
	query := connectionURL.Query()
	query.Set("replication", "database")
	connectionURL.RawQuery = query.Encode()
	return connectionURL.String(), nil
}

func replicationError(prefix string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42501" {
		return errors.New(prefix + ": permission denied; use a Neon Console, CLI, or API-created role with REPLICATION permission")
	}
	return errors.New(prefix)
}
