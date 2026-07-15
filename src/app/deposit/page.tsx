'use client';

import { useEffect } from 'react';
import { useRouter, useSearchParams } from 'next/navigation';

export default function DepositRedirect() {
  const router = useRouter();
  const searchParams = useSearchParams();

  useEffect(() => {
    const tab = searchParams.get('tab') || 'deposit';
    router.replace('/wallet?tab=' + tab);
  }, [router, searchParams]);

  return null;
}
