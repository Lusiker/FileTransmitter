// 平台检测工具
// 用于区分不同设备的传输策略

export type Platform = 'chrome' | 'safari' | 'firefox' | 'android' | 'ios'

/**
 * 检测当前平台
 */
export function detectPlatform(): Platform {
  const ua = navigator.userAgent

  // iOS 设备（iPhone、iPad）
  if (/iPad|iPhone|iPod/.test(ua)) {
    return 'ios'
  }

  // Android 设备
  if (/Android/.test(ua)) {
    return 'android'
  }

  // Safari（非 Chrome）
  // 注意：Chrome on Mac 也会包含 Safari 字样，需要排除
  if (/Safari/.test(ua) && !/Chrome/.test(ua) && !/Chromium/.test(ua)) {
    return 'safari'
  }

  // Firefox
  if (/Firefox/.test(ua)) {
    return 'firefox'
  }

  // 默认 Chrome/Edge
  return 'chrome'
}

/**
 * 检测是否支持 File System Access API
 * Chrome 86+ 支持，Safari/Firefox 不支持
 */
export function supportsFileSystemAccess(): boolean {
  return 'showSaveFilePicker' in window && 'showOpenFilePicker' in window
}

/**
 * 检测是否支持流式接收
 * iOS/Safari 不支持流式接收，只能逐文件下载
 */
export function supportsStreamingReceive(): boolean {
  const platform = detectPlatform()

  // iOS 和 Safari 不支持流式接收
  if (platform === 'ios' || platform === 'safari') {
    return false
  }

  // Android 和 Chrome 支持流式接收
  if (platform === 'android' || platform === 'chrome') {
    return supportsFileSystemAccess()
  }

  // Firefox 目前不支持 File System Access API
  if (platform === 'firefox') {
    return false
  }

  return false
}

/**
 * 检测是否是移动设备
 */
export function isMobile(): boolean {
  const platform = detectPlatform()
  return platform === 'ios' || platform === 'android'
}

/**
 * 检测是否是 iOS 设备
 */
export function isIOS(): boolean {
  return detectPlatform() === 'ios'
}

/**
 * 检测是否是 Safari 浏览器
 */
export function isSafari(): boolean {
  return detectPlatform() === 'safari' || detectPlatform() === 'ios'
}

/**
 * 获取平台友好名称
 */
export function getPlatformName(): string {
  const platform = detectPlatform()

  switch (platform) {
    case 'ios':
      return 'iOS Safari'
    case 'android':
      return 'Android'
    case 'safari':
      return 'Safari'
    case 'firefox':
      return 'Firefox'
    case 'chrome':
      return 'Chrome/Edge'
    default:
      return 'Unknown'
  }
}

/**
 * 获取传输模式描述
 */
export function getTransferModeDescription(): string {
  if (supportsStreamingReceive()) {
    return '流式接收（自动保存）'
  }
  return '逐文件下载（需手动保存）'
}