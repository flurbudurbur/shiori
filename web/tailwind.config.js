/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./src/**/*.{html,ts}",
  ],
  theme: {
    extend: {
      animation: {
        'fadeIn': 'fadeIn 0.5s ease-in-out',
      },
      keyframes: {
        fadeIn: {
          '0%': { opacity: '0' },
          '100%': { opacity: '1' },
        }
      },
      // Extended color palette with accessible combinations
      colors: {
        // Enhanced teal colors with better contrast
        teal: {
          300: '#5eead4', // Brighter for better contrast on dark backgrounds
          400: '#2dd4bf', // Adjusted for 4.5:1 contrast ratio on dark backgrounds
          500: '#14b8a6', // Base teal color
        },
        // Enhanced gray colors with better contrast
        gray: {
          100: '#f3f4f6', // Light text on dark backgrounds
          200: '#e5e7eb', // Light text on dark backgrounds
          300: '#d1d5db', // Improved contrast for text on dark backgrounds
          400: '#a3aab6', // Updated for 4.5:1 contrast ratio on dark backgrounds
          800: '#1f2937', // Dark background with good contrast for light text
          900: '#111827', // Darker background
          950: '#030712', // Darkest background
        },
      },
    },
  },
  plugins: [],
}