/** @type {import('next').NextConfig} */
const nextConfig = {
  reactStrictMode: true,
  async rewrites() {
    return [
      {
        // Redireciona /api/crm/* para o backend
        source: '/api/crm/:path*',
        destination: 'http://localhost:8081/:path*',
      },
      {
        // Redireciona /api/auth/* para o backend
        source: '/api/auth/:path*',
        destination: 'http://localhost:8081/auth/:path*',
      },
    ]
  },
}

module.exports = nextConfig
