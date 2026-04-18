<template>
  <el-alert
    v-if="showGuide"
    title="iPad 保存提示"
    type="info"
    show-icon
    :closable="false"
    class="ios-guide"
  >
    <template #default>
      <p>打开文件后，请按以下步骤保存：</p>
      <ol>
        <li>点击右上角 <strong>分享</strong> 按钮</li>
        <li>选择 <strong>存储到"文件"</strong></li>
        <li>选择保存位置</li>
      </ol>
    </template>
  </el-alert>
</template>

<script setup lang="ts">
import { isIOS } from '@/composables/useFileTransfer'

const showGuide = isIOS()
</script>

<style lang="scss" scoped>
.ios-guide {
  margin-bottom: 16px;

  p {
    margin-bottom: 8px;
  }

  ol {
    margin: 0;
    padding-left: 20px;

    li {
      margin-bottom: 4px;
    }
  }
}
</style>