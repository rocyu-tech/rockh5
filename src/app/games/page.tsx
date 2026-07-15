'use client';

import { useState } from 'react';
import { Gamepad2 } from 'lucide-react';
import Navbar from '@/components/Navbar';
import GameCategories from '@/components/GameCategories';
import GameGrid from '@/components/GameGrid';

export default function GamesPage() {
  const [activeCategory, setActiveCategory] = useState(0);

  return (
    <div>
      <Navbar
        onLoginClick={() => window.dispatchEvent(new CustomEvent('auth:logout'))}
        onRegisterClick={() => window.dispatchEvent(new CustomEvent('nav:open-register'))}
      />

      <main className="pt-14 px-4">
        {/* Page header */}
        <div className="flex items-center gap-2 mb-4">
          <Gamepad2 className="w-5 h-5 text-[#f5a623]" />
          <h1 className="text-lg font-bold text-white">Games</h1>
        </div>

        {/* Categories */}
        <GameCategories
          activeCategory={activeCategory}
          onCategoryChange={setActiveCategory}
        />

        {/* Game Grid */}
        <div className="mt-3">
          <GameGrid categoryId={activeCategory} />
        </div>
      </main>
    </div>
  );
}
