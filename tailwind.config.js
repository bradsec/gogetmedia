module.exports = {
  darkMode: 'class',
  content: [
    "./internal/ui/templates.go",
    "./web/src/**/*.{html,js,vue}",
    "./web/public/**/*.html"
  ],
  theme: {
    extend: {
      colors: {
        primary: {
          50: '#eff6ff',
          500: '#3b82f6',
          600: '#2563eb',
          700: '#1d4ed8',
          900: '#1e3a8a'
        }
      }
    }
  },
  plugins: []
}