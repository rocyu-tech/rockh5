import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import "./globals.css";
import { Toaster } from "@/components/ui/toaster";
import { Toaster as SonnerToaster } from "@/components/ui/sonner";
import BottomNav from "@/components/BottomNav";
import AppProvider from "@/components/AppProvider";
import AgeGate from "@/components/AgeGate";
import { I18nProvider } from "@/i18n/provider";

const geistSans = Geist({
  variable: "--font-geist-sans",
  subsets: ["latin"],
});

const geistMono = Geist_Mono({
  variable: "--font-geist-mono",
  subsets: ["latin"],
});

// P0-10 FIX: icon was previously hard-coded to a third-party CDN
// (https://z-cdn.chatglm.cn/z-ai/static/logo.svg). This creates:
//   - External dependency (if the CDN goes down, the icon disappears)
//   - Privacy leak (every page view pings the CDN with a Referer header)
//   - Branding inconsistency (the icon is the Z.ai logo, not RockGame's)
//
// Fix: point to /logo.svg in the public/ folder. If the file doesn't exist
// yet, browsers fall back to the default favicon — no external request.
export const metadata: Metadata = {
  title: "RockGame — Premium Gaming Platform",
  description: "Experience the thrill of premium online gaming. Slots, Live Casino, Sports, and more at RockGame.",
  keywords: ["RockGame", "casino", "gaming", "slots", "live casino", "online gaming"],
  icons: {
    icon: "/logo.svg",
  },
};

// P0-10 FIX: viewport was previously declared inline in <head> with
// `maximum-scale=1, user-scalable=no` which disables user zoom. This is an
// accessibility violation (WCAG 1.4.4) — users with low vision cannot zoom
// in to read content. The Next.js App Router recommends declaring viewport
// via the separate `viewport` export, which is also where safe-area insets
// for notched phones should go.
export const viewport: Viewport = {
  width: "device-width",
  initialScale: 1,
  viewportFit: "cover",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark" suppressHydrationWarning>
      <body className={`${geistSans.variable} ${geistMono.variable} antialiased bg-background text-foreground`}>
        <I18nProvider>
          <AppProvider>
            {children}
            <BottomNav />
            {/* P0-9: age gate — shown once per browser (cookie-backed). */}
            <AgeGate />
          </AppProvider>
        </I18nProvider>
        <Toaster />
        <SonnerToaster />
      </body>
    </html>
  );
}
