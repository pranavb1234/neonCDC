"use client";

export function DatabaseNode({ className = "" }: { className?: string }) {
  return (
    <span className={`inline-flex items-center align-middle ${className}`}>
      <svg
        className="h-10 w-12 sm:h-12 sm:w-14 md:h-14 md:w-16 drop-shadow-[0_0_15px_rgba(34,197,94,0.45)]"
        viewBox="0 0 68 54"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <defs>
          {/* Metallic body gradient */}
          <linearGradient id="dbBodyGrad" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="#475569" />
            <stop offset="35%" stopColor="#94a3b8" />
            <stop offset="70%" stopColor="#cbd5e1" />
            <stop offset="100%" stopColor="#334155" />
          </linearGradient>

          {/* Top disc cap gradient */}
          <linearGradient id="dbCapGrad" x1="0%" y1="0%" x2="0%" y2="100%">
            <stop offset="0%" stopColor="#cbd5e1" />
            <stop offset="100%" stopColor="#64748b" />
          </linearGradient>

          {/* LED Glow filter */}
          <filter id="ledGlow" x="-50%" y="-50%" width="200%" height="200%">
            <feGaussianBlur stdDeviation="1.5" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        {/* Database Cylinder - Tier 3 (Bottom) */}
        <path
          d="M 6 26 C 6 23 20 23 26 23 C 32 23 46 23 46 26 L 46 38 C 46 43 6 43 6 38 Z"
          fill="url(#dbBodyGrad)"
          stroke="#1e293b"
          strokeWidth="0.8"
        />
        <ellipse cx="26" cy="38" rx="20" ry="5.5" fill="#1e293b" opacity="0.4" />

        {/* Database Cylinder - Tier 2 (Middle) */}
        <path
          d="M 6 15 C 6 12 20 12 26 12 C 32 12 46 12 46 15 L 46 27 C 46 32 6 32 6 27 Z"
          fill="url(#dbBodyGrad)"
          stroke="#1e293b"
          strokeWidth="0.8"
        />
        <ellipse cx="26" cy="27" rx="20" ry="5.5" fill="#334155" opacity="0.3" />

        {/* Database Cylinder - Tier 1 (Top) */}
        <path
          d="M 6 6 C 6 2 20 2 26 2 C 32 2 46 2 46 6 L 46 16 C 46 21 6 21 6 16 Z"
          fill="url(#dbBodyGrad)"
          stroke="#1e293b"
          strokeWidth="0.8"
        />
        <ellipse cx="26" cy="6" rx="20" ry="5.5" fill="url(#dbCapGrad)" stroke="#475569" strokeWidth="0.8" />

        {/* Glowing Green LED Status Indicators */}
        {/* Top tier LED */}
        <rect
          x="12"
          y="9"
          width="4.5"
          height="2"
          rx="1"
          fill="#00ff88"
          filter="url(#ledGlow)"
        />
        <rect
          x="19"
          y="9"
          width="4.5"
          height="2"
          rx="1"
          fill="#00ff88"
          filter="url(#ledGlow)"
        />

        {/* Middle tier LED */}
        <rect
          x="12"
          y="19"
          width="5"
          height="2"
          rx="1"
          fill="#22c55e"
          filter="url(#ledGlow)"
        />
        <rect
          x="20"
          y="19"
          width="6"
          height="2"
          rx="1"
          fill="#22c55e"
          filter="url(#ledGlow)"
        />

        {/* Bottom tier LED */}
        <rect
          x="12"
          y="30"
          width="4"
          height="2"
          rx="1"
          fill="#10b981"
          filter="url(#ledGlow)"
        />
        <rect
          x="18"
          y="30"
          width="7"
          height="2"
          rx="1"
          fill="#34d399"
          filter="url(#ledGlow)"
        />

        {/* Cables / Wire connectors branching out from right flank into pipeline */}
        <path
          d="M 44 14 Q 50 14 54 18 Q 58 22 66 22"
          stroke="#10b981"
          strokeWidth="2"
          strokeLinecap="round"
          filter="url(#ledGlow)"
        />
        <path
          d="M 44 24 Q 48 24 52 28 Q 56 32 64 32"
          stroke="#22c55e"
          strokeWidth="1.8"
          strokeLinecap="round"
        />
        <path
          d="M 43 33 Q 48 33 52 38 Q 56 42 62 42"
          stroke="#34d399"
          strokeWidth="1.6"
          strokeLinecap="round"
        />

        {/* Cable terminal connectors */}
        <circle cx="66" cy="22" r="2" fill="#00ff88" />
        <circle cx="64" cy="32" r="1.8" fill="#22c55e" />
        <circle cx="62" cy="42" r="1.5" fill="#34d399" />
      </svg>
    </span>
  );
}
