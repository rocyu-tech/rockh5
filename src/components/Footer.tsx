'use client';

import { Separator } from '@/components/ui/separator';
import { Gamepad2, Mail, Shield, HelpCircle } from 'lucide-react';

const footerLinks = {
  'Games': ['Slots', 'Live Casino', 'Sports Betting', 'Table Games', 'Fishing', 'Crash Games'],
  'Support': ['Help Center', 'FAQ', 'Live Chat', 'Contact Us', 'Terms of Service', 'Privacy Policy'],
  'Company': ['About Us', 'Affiliate Program', 'Responsible Gaming', 'Fair Play', 'Licensing'],
};

const socialLinks = [
  { name: 'Telegram', icon: '✈️' },
  { name: 'Twitter', icon: '𝕏' },
  { name: 'Discord', icon: '💬' },
  { name: 'Instagram', icon: '📷' },
];

export default function Footer() {
  return (
    <footer className="mt-auto border-t border-[#f5a623]/10 bg-[#0a0a1a]">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-10 md:py-14">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-8 lg:gap-12">
          {/* Brand */}
          <div className="space-y-4">
            <div className="flex items-center gap-2">
              <div className="w-9 h-9 rounded-lg bg-gradient-to-br from-[#f5a623] to-[#e94560] flex items-center justify-center">
                <Gamepad2 className="w-5 h-5 text-white" />
              </div>
              <span className="text-xl font-bold text-gold-gradient">RockGame</span>
            </div>
            <p className="text-sm text-[#8892b0] leading-relaxed">
              The ultimate premium gaming platform. Experience world-class casino games, live dealers, and sports betting.
            </p>
            {/* Social Links */}
            <div className="flex gap-3">
              {socialLinks.map((social) => (
                <button
                  key={social.name}
                  className="w-9 h-9 rounded-lg bg-[#1a1a2e] border border-[#f5a623]/10 flex items-center justify-center hover:border-[#f5a623]/30 hover:bg-[#16213e] transition-all"
                  aria-label={social.name}
                >
                  <span className="text-sm">{social.icon}</span>
                </button>
              ))}
            </div>
          </div>

          {/* Links */}
          {Object.entries(footerLinks).map(([title, links]) => (
            <div key={title}>
              <h4 className="text-sm font-semibold text-[#f5a623] mb-4 uppercase tracking-wider">
                {title}
              </h4>
              <ul className="space-y-2.5">
                {links.map((link) => (
                  <li key={link}>
                    <button className="text-sm text-[#8892b0] hover:text-[#ccd6f6] transition-colors">
                      {link}
                    </button>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        <Separator className="my-8 bg-[#f5a623]/10" />

        {/* Bottom bar */}
        <div className="flex flex-col sm:flex-row items-center justify-between gap-4">
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-1.5 text-xs text-[#8892b0]">
              <Shield className="w-3.5 h-3.5 text-[#4ecdc4]" />
              <span>SSL Encrypted</span>
            </div>
            <div className="flex items-center gap-1.5 text-xs text-[#8892b0]">
              <HelpCircle className="w-3.5 h-3.5 text-[#f5a623]" />
              <span>18+ Only</span>
            </div>
            <div className="flex items-center gap-1.5 text-xs text-[#8892b0]">
              <Mail className="w-3.5 h-3.5 text-[#a855f7]" />
              <span>support@rockgame.com</span>
            </div>
          </div>
          <p className="text-xs text-[#8892b0]/60">
            © {new Date().getFullYear()} RockGame. All rights reserved. Play responsibly.
          </p>
        </div>
      </div>
    </footer>
  );
}
