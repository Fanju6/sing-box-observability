import { test, expect } from '@playwright/test'

const entryUrl = new URL('../../../../packaging/magisk/webroot/index.html', import.meta.url).href
const panelUrl = 'http://127.0.0.1:9095/'

test('KernelSU WebUI starts the service and opens the embedded panel', async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(window, 'ksu', {
      configurable: true,
      value: {
        exec(command: string) {
          return command.includes(' status ') ? 'sing-box-observability is running (PID 123)' : ''
        },
      },
    })
  })
  await page.route(panelUrl, (route) => route.fulfill({
    contentType: 'text/html',
    body: '<!doctype html><title>Observability panel</title><main>Panel</main>',
  }))

  await page.goto(entryUrl)

  await expect(page).toHaveURL(panelUrl)
  await expect(page).toHaveTitle('Observability panel')
})

test('KernelSU WebUI keeps a useful retry screen when the service fails', async ({ page }) => {
  await page.addInitScript(() => {
    Object.defineProperty(window, 'ksu', {
      configurable: true,
      value: { exec: () => 'sing-box-observability is stopped' },
    })
  })

  await page.goto(entryUrl)

  await expect(page.getByRole('status')).toContainText('服务没有成功启动')
  await expect(page.getByRole('button', { name: '重试' })).toBeVisible()
  await expect(page.getByRole('link', { name: '直接打开' })).toHaveAttribute('href', panelUrl)
})
