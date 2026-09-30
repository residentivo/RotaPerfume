/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  // Gera .next/standalone (server.js + node_modules minimos) para a imagem Docker.
  output: 'standalone',
};

module.exports = nextConfig;
