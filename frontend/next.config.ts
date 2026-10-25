import type { NextConfig } from "next";

const APP_VERSION = process.env.NEXT_PUBLIC_APP_VERSION || "dev"; // Default to "dev" if not set

const nextConfig: NextConfig = {
  trailingSlash: true,
  images: {
    remotePatterns: [
      {
        protocol: "http",
        hostname: "localhost",
        port: "8888",
        pathname: "/**",
      },
      {
        protocol: "http",
        hostname: "10.1.1.30",
        port: "8888",
        pathname: "/**",
      },
    ],
    localPatterns: [
      {
        pathname: "/api/images/**",
      },
    ],
  },
  allowedDevOrigins: ["localhost", "10.1.1.30"],
  reactStrictMode: false,
  env: {
    NEXT_PUBLIC_APP_VERSION: APP_VERSION,
  },
  experimental: {
    proxyTimeout: 300000,
  },
  output: "standalone",
  productionBrowserSourceMaps: true,
  poweredByHeader: false,

  async rewrites() {
    return [
      {
        source: "/api/:path*",
        destination: `http://localhost:8888/api/:path*`,
      },
      {
        source: "/swagger/:path*",
        destination: `http://localhost:8888/swagger/:path*`,
      },
    ];
  },
};

export default nextConfig;
