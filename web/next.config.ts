import type { NextConfig } from "next";
import { createMDX } from "fumadocs-mdx/next";

const nextConfig: NextConfig = {
  ...(process.env.NEXT_STATIC_EXPORT === "true" ? { output: "export" } : {}),
};

export default createMDX()(nextConfig);
