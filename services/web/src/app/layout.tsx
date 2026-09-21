import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";

const inter = Inter({
  variable: "--font-inter",
  subsets: ["latin"],
});

const jetbrainsMono = JetBrains_Mono({
  variable: "--font-mono",
  subsets: ["latin"],
});

export const metadata: Metadata = {
  title: "DocIntel | Enterprise Document Intelligence & Extraction",
  description: "Next-generation multimodal document intelligence powered by Gemini. Extract structured data with calibrated confidence, sub-second latency, and human-in-the-loop review.",
  keywords: ["document intelligence", "OCR", "data extraction", "Gemini", "invoice processing", "document parser"],
  openGraph: {
    title: "DocIntel | Enterprise Document Intelligence",
    description: "Instant multimodal document extraction with calibrated confidence scoring and automated review routing.",
    type: "website",
  },
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className="dark">
      <body className={`${inter.variable} ${jetbrainsMono.variable} font-sans antialiased bg-[#090d16] text-slate-100 min-h-screen selection:bg-indigo-500/30 selection:text-indigo-200`}>
        {children}
      </body>
    </html>
  );
}
