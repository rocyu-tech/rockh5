'use client';

import { useState } from 'react';
import { useAuthStore } from '@/store/auth';
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Eye, EyeOff, Loader2, Mail, Lock, UserPlus, Phone, User } from 'lucide-react';

interface RegisterModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  switchToLogin: () => void;
}

export default function RegisterModal({ open, onOpenChange, switchToLogin }: RegisterModalProps) {
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [phone, setPhone] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState('');
  const { register, isLoading, lastError } = useAuthStore();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');

    if (!email || !password || !confirmPassword) {
      setError('Please fill in all required fields');
      return;
    }

    if (password !== confirmPassword) {
      setError('Passwords do not match');
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

    if (!/[a-zA-Z]/.test(password) || !/\d/.test(password)) {
      setError('Password must contain at least one letter and one digit');
      return;
    }

    const success = await register({
      email,
      password,
      confirm_password: confirmPassword,
      phone: phone || undefined,
    });

    if (success) {
      onOpenChange(false);
      setEmail('');
      setPassword('');
      setConfirmPassword('');
      setPhone('');
      setError('');
    } else {
      setError(lastError || 'Registration failed. Please try again.');
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md bg-[#1e293b] border-[#f5a623]/40 text-[#ccd6f6] shadow-2xl shadow-[#f5a623]/10 backdrop-blur-xl">
        <DialogHeader>
          <DialogTitle className="text-center text-2xl font-bold">
            <span className="text-gold-gradient">Join RockGame</span>
          </DialogTitle>
          <DialogDescription className="text-center text-sm text-[#8892b0] mt-1">Create your account and start winning</DialogDescription>
        </DialogHeader>

        <form onSubmit={handleSubmit} className="space-y-4 mt-4">
          {/* Email */}
          <div className="space-y-2">
            <Label htmlFor="reg-email" className="text-[#8892b0] text-sm">
              Email <span className="text-[#e94560]">*</span>
            </Label>
            <div className="relative">
              <Mail className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#8892b0]" />
              <Input
                id="reg-email"
                type="email"
                placeholder="your@email.com"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="pl-10 pr-4 bg-[#16213e] border-[#f5a623]/20 placeholder-[#8892b0]/50 focus:border-[#f5a623]/50"
              />
            </div>
          </div>

          {/* Phone */}
          <div className="space-y-2">
            <Label htmlFor="reg-phone" className="text-[#8892b0] text-sm">
              Phone <span className="text-[#8892b0]/50">(optional)</span>
            </Label>
            <div className="relative">
              <Phone className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#8892b0]" />
              <Input
                id="reg-phone"
                type="tel"
                placeholder="+1234567890"
                value={phone}
                onChange={(e) => setPhone(e.target.value)}
                className="pl-10 pr-4 bg-[#16213e] border-[#f5a623]/20 placeholder-[#8892b0]/50 focus:border-[#f5a623]/50"
              />
            </div>
          </div>

          {/* Password */}
          <div className="space-y-2">
            <Label htmlFor="reg-password" className="text-[#8892b0] text-sm">
              Password <span className="text-[#e94560]">*</span>
            </Label>
            <div className="relative">
              <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#8892b0]" />
              <Input
                id="reg-password"
                type={showPassword ? 'text' : 'password'}
                placeholder="Min 6 characters"
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

          {/* Confirm Password */}
          <div className="space-y-2">
            <Label htmlFor="reg-confirm" className="text-[#8892b0] text-sm">
              Confirm Password <span className="text-[#e94560]">*</span>
            </Label>
            <div className="relative">
              <Lock className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[#8892b0]" />
              <Input
                id="reg-confirm"
                type={showPassword ? 'text' : 'password'}
                placeholder="Confirm your password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                className="pl-10 pr-4 bg-[#16213e] border-[#f5a623]/20 placeholder-[#8892b0]/50 focus:border-[#f5a623]/50"
              />
            </div>
          </div>

          {/* Error */}
          {error && (
            <p className="text-sm text-[#e94560] bg-[#e94560]/10 px-3 py-2 rounded-lg">{error}</p>
          )}

          {/* Terms */}
          <p className="text-xs text-[#8892b0] text-center">
            By registering, you agree to our{' '}
            <button type="button" className="text-[#f5a623] hover:underline">Terms of Service</button>{' '}
            and{' '}
            <button type="button" className="text-[#f5a623] hover:underline">Privacy Policy</button>.
          </p>

          {/* Submit */}
          <Button
            type="submit"
            disabled={isLoading}
            className="w-full bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold hover:from-[#ffd700] hover:to-[#f5a623] shadow-lg shadow-[#f5a623]/20 py-2.5"
          >
            {isLoading ? (
              <Loader2 className="w-4 h-4 animate-spin mr-2" />
            ) : (
              <UserPlus className="w-4 h-4 mr-2" />
            )}
            Create Account
          </Button>

          {/* Switch to login */}
          <p className="text-center text-sm text-[#8892b0]">
            Already have an account?{' '}
            <button
              type="button"
              onClick={switchToLogin}
              className="text-[#f5a623] font-semibold hover:underline"
            >
              Sign in
            </button>
          </p>
        </form>
      </DialogContent>
    </Dialog>
  );
}
