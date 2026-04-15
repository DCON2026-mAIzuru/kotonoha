import type { Metadata, Viewport } from 'next';
import './globals.css';

export const metadata: Metadata = {
  title: '言ノ葉',
  description: '盆栽と会話するチャットアプリ',
  manifest: '/manifest.webmanifest',
};

export const viewport: Viewport = {
  themeColor: '#f7f4ed',
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="ja">
      <body>{children}</body>
    </html>
  );
}
