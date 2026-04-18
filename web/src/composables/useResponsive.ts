import { ref, onMounted, onUnmounted, computed } from 'vue'

const breakpoints = {
  xs: 0,
  sm: 768,
  md: 1024,
  lg: 1280
}

export function useResponsive() {
  const width = ref(window.innerWidth)
  const height = ref(window.innerHeight)

  const breakpoint = computed(() => {
    const w = width.value
    if (w < breakpoints.sm) return 'xs'
    if (w < breakpoints.md) return 'sm'
    if (w < breakpoints.lg) return 'md'
    return 'lg'
  })

  const isMobile = computed(() => breakpoint.value === 'xs')
  const isTablet = computed(() => breakpoint.value === 'sm')
  const isDesktop = computed(() => breakpoint.value === 'md' || breakpoint.value === 'lg')
  const isLarge = computed(() => breakpoint.value === 'lg')

  const update = () => {
    width.value = window.innerWidth
    height.value = window.innerHeight
  }

  onMounted(() => {
    update()
    window.addEventListener('resize', update)
  })

  onUnmounted(() => {
    window.removeEventListener('resize', update)
  })

  return {
    width,
    height,
    breakpoint,
    isMobile,
    isTablet,
    isDesktop,
    isLarge
  }
}