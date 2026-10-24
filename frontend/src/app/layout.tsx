"use client";

import { ThemeProvider } from "@/providers/theme-provider";
import { Toaster } from "sonner";

import AppFooter from "@/components/layout/app-footer";
import { Navbar } from "@/components/layout/app-navbar";
import { JumpToTop } from "@/components/shared/jump-to-top";
import { ViewDensityProvider } from "@/components/shared/view-density-context";

import { gabarito } from "../../public/fonts/Gabarito";
import "./globals.css";

const APP_VERSION = process.env.NEXT_PUBLIC_APP_VERSION || "dev";

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <head>
        <link rel="manifest" href="/site.webmanifest" />
        <link rel="apple-touch-icon" href="/web-app-manifest-padded-192x192.png" />
        <meta name="apple-mobile-web-app-capable" content="yes" />
      </head>
      <body className={`${gabarito.className}`}>
        <ThemeProvider attribute="class" defaultTheme="dark" disableTransitionOnChange>
          <ViewDensityProvider>
            {/* Navbar */}
            <Navbar version={APP_VERSION} />

            {/* Main Content */}
            <main className="min-h-screen"> {children}</main>
          </ViewDensityProvider>

          {/* Footer */}
          <AppFooter version={APP_VERSION} />
          <Toaster richColors position="top-center" />
          {/* Floating buttons, a page's Refresh button goes on the right of Jump to Top */}
          <div id="floating-buttons" className="fixed bottom-5 right-3 z-100 flex items-center gap-2 sm:bottom-15">
            <JumpToTop />
          </div>
        </ThemeProvider>
      </body>
    </html>
  );
}
