'use client';

import { useState, useEffect, useCallback } from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { lobbyApi, type Banner } from '@/lib/api';
import { useApiStatusContext, getErrorMessage } from '@/lib/api-status';
import DemoBadge from '@/components/DemoBadge';

const placeholderBanners: Banner[] = [
  { id: 1, title: 'Welcome to RockGame', image_url: '/banner1.png', link_url: '', sort_order: 1, status: 1 },
  { id: 2, title: 'Mega Slots Tournament', image_url: '/banner2.png', link_url: '', sort_order: 2, status: 1 },
  { id: 3, title: 'Live Casino Experience', image_url: '/banner3.png', link_url: '', sort_order: 3, status: 1 },
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
      if (res.data?.data?.length) setBanners(res.data.data);
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

  const next = useCallback(() => goTo((current + 1) % banners.length), [current, banners.length, goTo]);
  const prev = useCallback(() => goTo((current - 1 + banners.length) % banners.length), [current, banners.length, goTo]);

  useEffect(() => {
    const timer = setInterval(next, 5000);
    return () => clearInterval(timer);
  }, [next]);

  return (
    <section className="relative w-full overflow-hidden rounded-xl">
      {usingDemo && <div className="absolute top-2 left-2 z-10"><DemoBadge show label="Demo" /></div>}
      <div className="relative w-full" style={{ aspectRatio: '16/9' }}>
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
              {/* Subtle pattern */}
              <div className="absolute inset-0 opacity-10">
                <div className="absolute top-3 right-4 w-20 h-20 rounded-full border-2 border-[#f5a623]/30" />
                <div className="absolute bottom-4 right-10 w-12 h-12 rounded-full border-2 border-[#e94560]/20" />
              </div>

              {/* Content - mobile optimized */}
              <div className="absolute inset-0 flex items-center px-4 sm:px-8 md:px-12">
                <div className="w-full">
                  <div className="inline-block px-2.5 py-0.5 rounded-full bg-[#f5a623]/20 text-[#f5a623] text-[10px] font-medium mb-2 sm:mb-3">
                    🎮 RockGame Exclusive
                  </div>
                  <h2 className="text-lg sm:text-2xl md:text-3xl lg:text-4xl font-bold text-white mb-1.5 sm:mb-3 leading-tight line-clamp-2">
                    {banner.title}
                  </h2>
                  <p className="text-xs sm:text-sm text-[#8892b0] mb-3 sm:mb-5 line-clamp-2">
                    Experience the thrill of premium gaming with exclusive bonuses. Play now and win big!
                  </p>
                  <button className="px-4 sm:px-6 py-2 sm:py-2.5 bg-gradient-to-r from-[#f5a623] to-[#e8a910] text-[#0a0a1a] font-semibold rounded-lg text-xs sm:text-sm shadow-lg shadow-[#f5a623]/20 active:scale-95 transition-transform">
                    Play Now →
                  </button>
                </div>
              </div>
            </div>
          </div>
        ))}

        {/* Prev/Next - small buttons, kept inside safe area */}
        <button
          onClick={prev}
          className="absolute left-1.5 top-1/2 -translate-y-1/2 z-10 w-7 h-7 sm:w-9 sm:h-9 rounded-full bg-black/40 backdrop-blur-sm border border-white/10 flex items-center justify-center text-white/70 active:bg-black/60 transition-all"
          aria-label="Previous banner"
        >
          <ChevronLeft className="w-3.5 h-3.5 sm:w-4 sm:h-4" />
        </button>
        <button
          onClick={next}
          className="absolute right-1.5 top-1/2 -translate-y-1/2 z-10 w-7 h-7 sm:w-9 sm:h-9 rounded-full bg-black/40 backdrop-blur-sm border border-white/10 flex items-center justify-center text-white/70 active:bg-black/60 transition-all"
          aria-label="Next banner"
        >
          <ChevronRight className="w-3.5 h-3.5 sm:w-4 sm:h-4" />
        </button>

        {/* Dots */}
        <div className="absolute bottom-2.5 left-1/2 -translate-x-1/2 z-10 flex gap-1.5">
          {banners.map((_, index) => (
            <button
              key={index}
              onClick={() => goTo(index)}
              className={`h-1.5 rounded-full transition-all duration-300 ${
                current === index
                  ? 'w-5 bg-[#f5a623]'
                  : 'w-1.5 bg-white/30'
              }`}
              aria-label={`Go to banner ${index + 1}`}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
