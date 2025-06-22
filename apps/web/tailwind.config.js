/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./pages/**/*.{js,ts,jsx,tsx,mdx}",
    "./components/**/*.{js,ts,jsx,tsx,mdx}",
    "./app/**/*.{js,ts,jsx,tsx,mdx}",
    "./src/**/*.{js,ts,jsx,tsx,mdx}",
  ],
  theme: {
    extend: {
      colors: {
        teal: '#073E3A',
        lime: '#E6EE91',
        'lime-tint': '#F3F6D6',
        cream: '#F3EEDC',
        ink: '#16211F',
        body: '#45514E',
        muted: '#5B6B68',
        rule: '#DFE4E2',
      },
      fontFamily: {
        sans: ['"Segoe UI"', '"Helvetica Neue"', 'Arial', 'sans-serif'],
        serif: ['Georgia', 'Cambria', '"Times New Roman"', 'serif'],
      },
    },
  },
  plugins: [],
};
