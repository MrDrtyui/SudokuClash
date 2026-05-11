/** @type {import('tailwindcss').Config} */
export default {
  content: ["./index.html", "./src/**/*.{js,jsx}"],
  theme: {
    extend: {
      colors: {
        sand: "#F6F0E6",
        ink: "#161616",
        coral: "#D95D39",
        olive: "#5E6C4D",
        wheat: "#E9D8B4",
        slatewarm: "#655C55"
      },
      boxShadow: {
        card: "0 14px 40px rgba(22, 22, 22, 0.08)"
      },
      borderRadius: {
        "4xl": "2rem"
      }
    }
  },
  plugins: []
};

