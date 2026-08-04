'use client';

import { useState } from 'react';
import { useAuthStore } from '@/store/auth';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Eye, EyeOff, Loader2, Phone, Lock, LogIn } from 'lucide-react';

interface LoginModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  switchToRegister: () => void;
}

const COUNTRY_CODES = [
  { code: '+63', label: '+63' },
  { code: '+86', label: '+86' },
  { code: '+1', label: '+1' },
  { code: '+44', label: '+44' },
  { code: '+81', label: '+81' },
  { code: '+82', label: '+82' },
  { code: '+66', label: '+66' },
  { code: '+84', label: '+84' },
  { code: '+91', label: '+91' },
  { code: '+62', label: '+62' },
  { code: '+60', label: '+60' },
  { code: '+855', label: '+855' },
];

export default function LoginModal({ open, onOpenChange, switchToRegister }: LoginModalProps) {
  const [countryCode, setCountryCode] = useState('+63');
  const [phone, setPhone] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState('');
  const { login, isLoading, lastError } = useAuthStore();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!phone || !password) {
      setError('Please fill in all fields');
      return;
    }

    if (!/^\d{7,15}$/.test(phone)) {
      setError('Please enter a valid phone number (7-15 digits)');
      return;
    }

    if (password.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }

    if (!/[a-zA-Z]/.test(password) || !/[0-9]/.test(password)) {
      setError('Password must contain both letters and numbers');
      return;
    }

    const success = await login(`${countryCode}${phone}`, password);
    if (success) {
      onOpenChange(false);
      setPhone('');
      setPassword('');
      setError('');
    } else {
      setError(lastError || 'Invalid phone number or password');
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md bg-[#1e293b] border-[#f5a623]/40 text-[#ccd6f6] shadow-2xl shadow-[#f5a623]/10 backdrop-blur-xl">
        <DialogHeader>
          <DialogTitle className="text-center text-2xl font-bold">
            <span className="text-gold-gradient">Welcome Back</span>
          </DialogTitle>
          <DialogDescription className="text-center text-sm text-[#8892b0] mt-1">Sign in to your RockGame account</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 mt-4">
          {/* Phone */}
          <div className="space-y-2">
            <Label htmlFor="login-phone" className="text-[#8892b0] text-sm">
              Phone Number
            </Label>
            <div className="flex gap-2">
              <select
                value={countryCode}
                onChange={(e) => setCountryCode(e.target.value)}
                className="w-24 shrink-0 rounded-lg bg-[#16213e] border-[#f5a623]/20 text-[#ccd6f6] text-sm px-2 py-2.5 focus:outline-none focus:border-[#f5a623]/50 appearance-none text-center cursor-pointer"
              >
                {COUNTRY_CODES.map((c) => (
                  <option key={c.code} value={c.code} className="bg-[#16213e] text-[#ccd6f6]">
                    {c.label}
                  </option>
                ))}
              </select>
              <div className="relative flex-1">
                <Phone className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#8892b0]" />
                <Input
                  id="login-phone"
                  type="tel"
                  placeholder="Enter your phone number"
                  value={phone}
                  onChange={(e) => setPhone(e.target.value.replace(/\D/g, ''))}
                  className="pl-10 pr-4 bg-[#16213e] border-[#f5a623]/20 placeholder-[#8892b0]/50 focus:border-[#f5a623]/50"
                />
              </div>
            </div>
          </div>

          {/* Password */}
          <div className="space-y-2">
            <Label htmlFor="login-password" className="text-[#8892b0] text-sm">
              Password
            </Label>
            <div className="relative">
              <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#8892b0]" />
              <Input
                id="login-password"
                type={showPassword ? 'text' : 'password'}
                placeholder="Enter your password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="pl-10 pr-10 bg-[#16213e] border-[#f5a623]/20 placeholder-[#8892b0]/50 focus:border-[#f5a623]/50"
              />
              <button
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-[#8892b0] hover:text-[#ccd6f6] transition-colors"
              >
                {showPassword ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
              </button>
            </div>
          </div>

          {/* Error */}
          {error && (
            <p className="text-sm text-[#e94560] bg-[#e94560]/10 px-3 py-2 rounded-lg">{error}</p>
          )}

          {/* Submit */}
          <Button
            type="submit"
            disabled={isLoading}
            className="w-full bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold hover:from-[#ffd700] hover:to-[#f5a623] shadow-lg shadow-[#f5a623]/20 py-2.5"
          >
            {isLoading ? (
              <Loader2 className="w-4 h-4 animate-spin mr-2" />
            ) : (
              <LogIn className="w-4 h-4 mr-2" />
            )}
            Sign In
          </Button>

          {/* Switch to register */}
          <p className="text-center text-sm text-[#8892b0]">
            Don&apos;t have an account?{' '}
            <button
              type="button"
              onClick={switchToRegister}
              className="text-[#f5a623] font-semibold hover:underline"
            >
              Register now
            </button>
          </p>
        </form>
      </DialogContent>
    </Dialog>
  );
}
