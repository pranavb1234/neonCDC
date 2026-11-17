"use client";

export function HeroCircuit() {
  return (
    <div className="pointer-events-none absolute inset-0 overflow-hidden select-none">
      {/* Radial soft emerald glow behind the main hero text */}
      <div className="absolute top-1/4 left-1/3 -translate-x-1/2 -translate-y-1/2 w-[520px] h-[320px] bg-emerald-500/15 blur-[120px] rounded-full" />
      <div className="absolute top-1/3 right-1/4 w-[400px] h-[280px] bg-emerald-600/10 blur-[140px] rounded-full" />

      {/* SVG Circuit Traces & Data Pipelines */}
      <svg
        className="absolute top-0 right-0 h-full w-full opacity-60 md:opacity-85"
        viewBox="0 0 1200 650"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        preserveAspectRatio="xMidYMid slice"
      >
        <defs>
          <linearGradient id="traceGrad1" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="#10b981" stopOpacity="0.2" />
            <stop offset="50%" stopColor="#22c55e" stopOpacity="0.8" />
            <stop offset="100%" stopColor="#10b981" stopOpacity="0.3" />
          </linearGradient>

          <linearGradient id="traceGrad2" x1="0%" y1="0%" x2="100%" y2="100%">
            <stop offset="0%" stopColor="#059669" stopOpacity="0.1" />
            <stop offset="40%" stopColor="#10b981" stopOpacity="0.7" />
            <stop offset="100%" stopColor="#34d399" stopOpacity="0.2" />
          </linearGradient>

          <filter id="neonGlow" x="-20%" y="-20%" width="140%" height="140%">
            <feGaussianBlur stdDeviation="3.5" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>

          <marker
            id="arrowhead-green"
            markerWidth="8"
            markerHeight="8"
            refX="6"
            refY="4"
            orient="auto"
          >
            <polygon points="0 1, 7 4, 0 7" fill="#22c55e" />
          </marker>
        </defs>

        {/* Subtle coordinate grid lines */}
        <g stroke="rgba(16, 185, 129, 0.08)" strokeWidth="1">
          {Array.from({ length: 15 }).map((_, i) => (
            <line key={`v-${i}`} x1={i * 80} y1="0" x2={i * 80} y2="650" />
          ))}
          {Array.from({ length: 9 }).map((_, i) => (
            <line key={`h-${i}`} x1="0" y1={i * 80} x2="1200" y2={i * 80} />
          ))}
        </g>

        {/* Circuit Pipeline Paths (matching Reference Image 1) */}
        {/* Track 1 - Upper flow */}
        <path
          d="M 460 210 L 640 210 Q 660 210 660 230 L 660 290 Q 660 310 680 310 L 920 310 Q 940 310 940 330 L 940 370 L 1180 370"
          stroke="url(#traceGrad1)"
          strokeWidth="2.5"
          filter="url(#neonGlow)"
          fill="none"
        />
        {/* Track 1 Arrowhead */}
        <polygon points="860,307 870,310 860,313" fill="#22c55e" filter="url(#neonGlow)" />
        <polygon points="1080,367 1090,370 1080,373" fill="#22c55e" filter="url(#neonGlow)" />

        {/* Track 2 - Middle flow with dashed segment */}
        <path
          d="M 520 250 L 720 250 Q 740 250 740 270 L 740 340 Q 740 360 760 360 L 960 360 Q 980 360 980 380 L 980 430 L 1190 430"
          stroke="#10b981"
          strokeWidth="1.8"
          strokeOpacity="0.7"
          fill="none"
        />
        <path
          d="M 770 360 L 910 360"
          stroke="#34d399"
          strokeWidth="2.2"
          strokeDasharray="6 6"
          fill="none"
        />
        <polygon points="900,357 910,360 900,363" fill="#34d399" />

        {/* Track 3 - Lower branch flowing downward */}
        <path
          d="M 680 310 L 780 310 Q 800 310 800 330 L 800 480 Q 800 500 820 500 L 1050 500 Q 1070 500 1070 520 L 1070 560 L 1190 560"
          stroke="url(#traceGrad2)"
          strokeWidth="2"
          fill="none"
        />
        <polygon points="760,307 770,310 760,313" fill="#22c55e" />
        <polygon points="980,497 990,500 980,503" fill="#22c55e" />

        {/* Track 4 - Thin high-tech parallel bus */}
        <path
          d="M 740 340 L 740 450 Q 740 470 760 470 L 880 470"
          stroke="rgba(16, 185, 129, 0.4)"
          strokeWidth="1.2"
          strokeDasharray="4 4"
          fill="none"
        />

        {/* Moving data packets along paths */}
        <circle r="3.5" fill="#a7f3d0" filter="url(#neonGlow)">
          <animateMotion
            path="M 460 210 L 640 210 Q 660 210 660 230 L 660 290 Q 660 310 680 310 L 920 310 Q 940 310 940 330 L 940 370 L 1180 370"
            dur="6s"
            repeatCount="indefinite"
          />
        </circle>

        <circle r="3" fill="#34d399" filter="url(#neonGlow)">
          <animateMotion
            path="M 520 250 L 720 250 Q 740 250 740 270 L 740 340 Q 740 360 760 360 L 960 360 Q 980 360 980 380 L 980 430 L 1190 430"
            dur="8s"
            begin="2s"
            repeatCount="indefinite"
          />
        </circle>

        {/* Glowing Circuit Junction Nodes */}
        <g>
          <circle cx="660" cy="230" r="3" fill="#22c55e" />
          <circle cx="680" cy="310" r="4" fill="#34d399" filter="url(#neonGlow)" />
          <circle cx="740" cy="270" r="3" fill="#10b981" />
          <circle cx="940" cy="330" r="3.5" fill="#22c55e" />
          <circle cx="980" cy="380" r="3" fill="#10b981" />
          <circle cx="820" cy="500" r="3.5" fill="#34d399" />
          <circle cx="1070" cy="520" r="3" fill="#22c55e" />
        </g>

        {/* High-tech star sparkles (✦) */}
        <g fill="#22c55e" opacity="0.6">
          <path d="M 580 180 Q 580 190 570 190 Q 580 190 580 200 Q 580 190 590 190 Q 580 190 580 180 Z" />
          <path d="M 850 240 Q 850 248 842 248 Q 850 248 850 256 Q 850 248 858 248 Q 850 248 850 240 Z" />
          <path d="M 1030 460 Q 1030 468 1022 468 Q 1030 468 1030 476 Q 1030 468 1038 468 Q 1030 468 1030 460 Z" />
        </g>
      </svg>
    </div>
  );
}
