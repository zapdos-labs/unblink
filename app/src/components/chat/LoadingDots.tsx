export default function LoadingDots() {
  return (
    <div class="flex gap-1">
      <div class="w-2 h-2 bg-neutral-500 rounded-full animate-pulse"></div>
      <div class="w-2 h-2 bg-neutral-500 rounded-full animate-pulse" style={{"animation-delay": "0.2s"}}></div>
      <div class="w-2 h-2 bg-neutral-500 rounded-full animate-pulse" style={{"animation-delay": "0.4s"}}></div>
    </div>
  );
}
