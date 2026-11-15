export type CdcEvent = {
  id: string;
  table: string;
  operation: "insert" | "update" | "delete";
  commit_lsn: string;
  data: Record<string, unknown>;
  before?: Record<string, unknown>;
  after?: Record<string, unknown>;
  created_at: string;
  stream_id: string;
};
