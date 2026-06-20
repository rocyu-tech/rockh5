'use client';

import { useState, useEffect, useCallback } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { lobbyApi, type Banner } from '@/lib/api';
import { useApiStatusContext, getErrorMessage } from '@/lib/api-status';
import DemoBadge from '@/components/DemoBadge';

const placeholderBanners: Banner[] = [
  {
    id: 1,
    title: 'Welcome to RockGame',
    image_url: '/banner1.png',
    link_url: '',
    sort_order: 1,
    status: 1,
  },
  {
    id: 2,
    title: 'Mega Slots Tournament',
    image_url: '/banner2.png',
    link_url: '',
    sort_order: 2,
    status: 1,
  },
  {
    id: 3,
    title: 'Live Casino Experience',
    image_url: '/banner3.png',
    link_url: '',
    sort_order: 3,
    status: 1,
  },
];

const bannerGradients = [
  'linear-gradient(135deg, #1a1a2e 0%, #16213e 40%, #0f3460 100%)',
  'linear-gradient(135deg, #1a0a2e 0%, #2d1b69 40%, #0f3460 100%)',
  'linear-gradient(135deg, #2e1a1a 0%, #3d1616 40%, #0f3460 100%)',
];

export default function BannerCarousel() {
  const [banners, setBanners] = useState<Banner[]>(placeholderBanners);
  const [current, setCurrent] = useState(0);
  const [isTransitioning, setIsTransitioning] = useState(false);
  const [usingDemo, setUsingDemo] = useState(false);
  const apiStatus = useApiStatusContext();

  useEffect(() => {
    lobbyApi.getBanners().then((res) => {
      if (res.data?.data?.length) {
        setBanners(res.data.data);
      }
    }).catch((err) => {
      setUsingDemo(true);
      apiStatus.markFailed('lobby/banners', getErrorMessage(err));
    });
  }, []);

  const goTo = useCallback((index: number) => {
    if (isTransitioning) return;
    setIsTransitioning(true);
    setCurrent(index);
    setTimeout(() => setIsTransitioning(false), 500);
  }, [isTransitioning]);

  const next = useCallback(() => {
    goTo((current + 1) % banners.length);
  }, [current, banners.length, goTo]);

  const prev = useCallback(() => {
    goTo((current - 1 + banners.length) % banners.length);
  }, [current, banners.length, goTo]);

  // Auto-play
  useEffect(() => {
    const timer = setInterval(next, 5000);
    return () => clearInterval(timer);
  }, [next]);

  return (
    <section className="relative w-full overflow-hidden rounded-xl mt-16 md:mt-20">
      {usingDemo && <div className="absolute top-3 left-3 z-10"><DemoBadge show label="Demo Banners" /></div>}
      <div className="relative aspect-[16/6] sm:aspect-[16/7] md:aspect-[16/8] bg-[#1a1a2e]">
        {banners.map((banner, index) => (
          <div
            key={banner.id}
            className="absolute inset-0 transition-opacity duration-500"
            style={{
              opacity: current === index ? 1 : 0,
              zIndex: current === index ? 1 : 0,
            }}
          >
            {/* Banner Background */}
            <div
              className="absolute inset-0"
              style={{ background: bannerGradients[index % bannerGradients.length] }}
            >
              {/* Pattern overlay */}
              <div className="absolute inset-0 opacity-10">
                <div className="absolute top-4 right-8 w-32 h-32 md:w-48 md:h-48 rounded-full border-2 border-[#f5a623]/30" />
                <div className="absolute bottom-8 right-16 w-20 h-20 md:w-32 md:h-32 rounded-full border-2 border-[#e94560]/20" />
                <div className="absolute top-1/2 left-8 w-16 h-16 md:w-24 md:h-24 rounded-full border border-[#f5a623]/20" />
              </div>

              {/* Content */}
              <div className="absolute inset-0 flex items-center px-8 sm:px-12 md:px-16">
                <div className="max-w-lg">
                  <div className="inline-block px-3 py-1 rounded-full bg-[#f5a623]/20 text-[#f5a623] text-xs font-medium mb-3">
                    🎮 RockGame Exclusive
                  </div>
                  <h2 className="text-2xl sm:text-3xl md:text-4xl lg:text-5xl font-bold text-white mb-3 leading-tight">
                    {banner.title}
                  </h2>
                  <p className="text-sm sm:text-base text-[#8892b0] mb-6 max-w-md">
                    Experience the thrill of premium gaming with exclusive bonuses and rewards. Play now and win big!
                  </p>
                  <button className="px-6 py-2.5 bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold rounded-lg hover:from-[#ffd700] hover:to-[#f5a623] transition-all shadow-lg shadow-[#f5a623]/20 text-sm">
                    Play Now →
                  </button>
                </div>
              </div>

              {/* Decorative elements on right */}
              <div className="hidden md:flex absolute right-8 lg:right-16 top-1/2 -translate-y-1/2">
                <div className="w-48 lg:w-64 h-48 lg:h-64 rounded-2xl border-2 border-[#f5a623]/20 bg-gradient-to-br from-[#f5a623]/5 to-transparent flex items-center justify-center">
                  <div className="w-24 lg:w-32 h-24 lg:h-32 rounded-xl bg-gradient-to-br from-[#f5a623]/10 to-[#e94560]/10 border border-[#f5a623]/10 flex items-center justify-center">
                    <GameIcon />
                  </div>
                </div>
              </div>
            </div>
          </div>
        ))}

        {/* Prev/Next buttons */}
        <button
          onClick={prev}
          className="absolute left-2 sm:left-4 top-1/2 -translate-y-1/2 z-10 w-11 h-11 rounded-full bg-black/40 backdrop-blur-sm border border-white/10 flex items-center justify-center text-white/70 hover:text-white hover:bg-black/60 transition-all"
          aria-label="Previous banner"
        >
          <ChevronLeft className="w-4 h-4 sm:w-5 sm:h-5" />
        </button>
        <button
          onClick={next}
          className="absolute right-2 sm:right-4 top-1/2 -translate-y-1/2 z-10 w-11 h-11 rounded-full bg-black/40 backdrop-blur-sm border border-white/10 flex items-center justify-center text-white/70 hover:text-white hover:bg-black/60 transition-all"
          aria-label="Next banner"
        >
          <ChevronRight className="w-4 h-4 sm:w-5 sm:h-5" />
        </button>

        {/* Dots */}
        <div className="absolute bottom-3 sm:bottom-4 left-1/2 -translate-x-1/2 z-10 flex gap-2">
          {banners.map((_, index) => (
            <button
              key={index}
              onClick={() => goTo(index)}
              className={`h-1.5 rounded-full transition-all duration-300 ${
                current === index
                  ? 'w-6 bg-[#f5a623]'
                  : 'w-1.5 bg-white/30 hover:bg-white/50'
              }`}
              aria-label={`Go to banner ${index + 1}`}
            />
          ))}
        </div>
      </div>
    </section>
  );
}

function GameIcon() {
  return (
    <svg viewBox="0 0 64 64" className="w-16 h-16 lg:w-20 lg:h-20 text-[#f5a623]/60">
      <path d="M16 8 L48 8 L56 24 L56 48 L40 56 L24 56 L8 48 L8 24 Z" fill="none" stroke="currentColor" strokeWidth="2" />
      <circle cx="20" cy="32" r="4" fill="currentColor" />
      <circle cx="44" cy="32" r="4" fill="currentColor" />
      <circle cx="32" cy="24" r="4" fill="currentColor" />
      <circle cx="32" cy="40" r="4" fill="currentColor" />
      <path d="M20 32 L32 24" stroke="currentColor" strokeWidth="1" />
      <path d="M44 32 L32 24" stroke="currentColor" strokeWidth="1" />
      <path d="M20 32 L32 40" stroke="currentColor" strokeWidth="1" />
      <path d="M44 32 L32 40" stroke="currentColor" strokeWidth="1" />
    </svg>
  );
}
