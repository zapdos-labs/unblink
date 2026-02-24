import { useIsPortrait } from './useIsPortrait';

export const useMaxWidth = () => {
  const isPortrait = useIsPortrait();

  return () => (isPortrait() ? 'max-w-full' : 'max-w-lg');
};
