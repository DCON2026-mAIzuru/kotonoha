import type { Config } from 'tailwindcss';

const config: Config = {
  content: ['./src/**/*.{js,ts,jsx,tsx,mdx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['CopyRight', 'Emoji', 'Hiragino Sans', 'BIZ UDPGothic Alphabet', 'BIZ UDGothic', 'Meiryo', 'sans-serif'],
      },
      colors: {
        brand: {
          green: '#00b900',
          pink: '#ff0077',
        },
        surface: {
          base: '#000000',
          card: '#1a1a1a',
          elevated: '#212121',
        },
      },
      boxShadow: {
        level1: '0 2px 4px rgba(0,0,0,0.5)',
        level2: '0 4px 12px rgba(0,0,0,0.6)',
      },
    },
  },
  plugins: [],
};

export default config;
