import { createSignal, onCleanup, onMount } from 'solid-js';

export const useIsPortrait = () => {
  const [isPortrait, setIsPortrait] = createSignal(false);

  const updateOrientation = () => {
    setIsPortrait(window.matchMedia('(orientation: portrait)').matches);
  };

  onMount(() => {
    updateOrientation();

    const mediaQuery = window.matchMedia('(orientation: portrait)');
    mediaQuery.addEventListener('change', updateOrientation);

    onCleanup(() => {
      mediaQuery.removeEventListener('change', updateOrientation);
    });
  });

  return isPortrait;
};
