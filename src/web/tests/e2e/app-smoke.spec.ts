import { test, expect } from '@playwright/test'

test('overview page loads and renders content', async ({ page }) => {
  await page.goto('/')
  await page.waitForLoadState('networkidle')
  await expect(page).toHaveURL(/\/overview$/)
  await expect(page.locator('main')).toBeVisible({ timeout: 15000 })
  await expect(page.locator('h1')).toBeVisible()
  await expect(page.getByText(/Diagnostics|诊断/)).toBeVisible()
  const wideCards = page.locator('[data-dashboard-wide="true"]')
  await expect(wideCards).toHaveCount(2)
  for (const card of await wideCards.all()) {
    expect(await card.evaluate((element) => getComputedStyle(element).gridColumn)).toBe('1 / -1')
  }
})

test('trends page loads', async ({ page }) => {
  await page.goto('/trends')
  await page.waitForLoadState('networkidle')
  await expect(page.locator('main')).toBeVisible({ timeout: 15000 })
  await expect(page.locator('h1')).toBeVisible()
  const list = page.getByRole('tablist').first()
  const active = list.getByRole('tab', { selected: true })
  const styles = await active.evaluate((element) => {
    const activeStyle = getComputedStyle(element)
    const listStyle = getComputedStyle(element.parentElement!)
    return {
      activeRadius: activeStyle.borderRadius,
      activeFontSize: activeStyle.fontSize,
      listHeight: listStyle.height,
      listRadius: listStyle.borderRadius,
    }
  })
  expect(styles).toEqual({
    activeRadius: '8px',
    activeFontSize: '13px',
    listHeight: page.viewportSize()!.width < 720 ? '36px' : '30px',
    listRadius: '10px',
  })
  const rangeSelect = page.getByRole('combobox').first()
  expect(await rangeSelect.evaluate((element) => getComputedStyle(element).fontSize)).toBe(
    page.viewportSize()!.width < 720 ? '16px' : '13px',
  )
  const metrics = page.locator('[data-summary-metric]')
  await expect(metrics).toHaveCount(3)
  const metricWidths = await metrics.evaluateAll((elements) => elements.map((element) => element.getBoundingClientRect().width))
  if (page.viewportSize()!.width < 640 && page.viewportSize()!.width > 360) {
    expect(metricWidths[2]).toBeGreaterThan(metricWidths[0] * 1.8)
  } else {
    expect(Math.abs(metricWidths[2] - metricWidths[0])).toBeLessThan(1)
  }
  for (const value of await page.locator('[data-summary-value]').all()) {
    expect(await value.evaluate((element) => getComputedStyle(element).textOverflow)).not.toBe('ellipsis')
  }
})

test('connections page loads', async ({ page }) => {
  await page.goto('/connections')
  await page.waitForLoadState('networkidle')
  await expect(page.locator('main')).toBeVisible({ timeout: 15000 })
  await expect(page.locator('h1')).toBeVisible()
  const header = page.locator('h1').locator('..')
  const content = header.locator('xpath=following-sibling::*[1]')
  const gap = await header.evaluate((element) => {
    const next = element.nextElementSibling
    return next ? next.getBoundingClientRect().top - element.getBoundingClientRect().bottom : -1
  })
  expect(gap).toBe(page.viewportSize()!.width < 720 ? 16 : 24)
  await expect(content).toBeVisible()
  const search = page.locator('main input').first()
  expect(await search.evaluate((element) => getComputedStyle(element).fontSize)).toBe(
    page.viewportSize()!.width < 720 ? '16px' : '13px',
  )
})

test('rankings page loads', async ({ page }) => {
  await page.goto('/rankings')
  await page.waitForLoadState('networkidle')
  await expect(page.locator('main')).toBeVisible({ timeout: 15000 })
  await expect(page.locator('h1')).toBeVisible()
})

test('settings page loads', async ({ page }) => {
  await page.goto('/settings')
  await page.waitForLoadState('networkidle')
  await expect(page.locator('main')).toBeVisible({ timeout: 15000 })
  await expect(page.locator('h1')).toBeVisible()
  await expect(page.getByText(/Cursor Pagination|游标分页/)).toBeVisible()
  await expect(page.locator('section > h2').first().locator('svg')).toHaveCount(0)
  const header = page.locator('h1').locator('..')
  const gap = await header.evaluate((element) => {
    const next = element.nextElementSibling
    return next ? next.getBoundingClientRect().top - element.getBoundingClientRect().bottom : -1
  })
  expect(gap).toBe(page.viewportSize()!.width < 720 ? 16 : 24)
})

test('responsive navigation matches the viewport', async ({ page }) => {
  await page.goto('/overview')
  await page.waitForLoadState('networkidle')
  const viewport = page.viewportSize()!
  const navigation = page.getByRole('navigation', { name: /Main navigation|主导航/ })
  const sidebar = page.locator('aside')
  if (viewport.width < 720) {
    await expect(page.locator('header')).toHaveText('sing-box-observability')
    await expect(page.locator('nav')).toHaveCount(1)
    const toggle = page.getByRole('button', { name: /Toggle navigation|切换导航/ })
    await expect(toggle).toBeVisible()
    await expect(sidebar).toHaveAttribute('data-open', 'false')
    const closedBox = await sidebar.boundingBox()
    expect(closedBox).not.toBeNull()
    expect(closedBox!.x + closedBox!.width).toBeLessThanOrEqual(1)
    await toggle.click()
    await expect(sidebar).toHaveAttribute('data-open', 'true')
    await expect(navigation).toBeVisible()
    await expect(navigation.getByRole('link')).toHaveCount(5)
    await expect.poll(async () => (await sidebar.boundingBox())?.x ?? -1).toBeGreaterThanOrEqual(0)
  } else {
    await expect(sidebar).toBeVisible()
    const box = await sidebar.boundingBox()
    expect(box?.width).toBe(232)
  }
})

test('overview dashboard items can be configured', async ({ page }) => {
  await page.goto('/overview')
  await page.waitForLoadState('networkidle')
  await page.getByRole('button', { name: /Dashboard Items|仪表项目/ }).click()
  const dialog = page.getByRole('dialog')
  await expect(dialog).toBeVisible()
  const dialogStyle = await dialog.evaluate((element) => {
    const style = getComputedStyle(element)
    return {
      width: element.getBoundingClientRect().width,
      paddingTop: style.paddingTop,
      paddingRight: style.paddingRight,
      radius: style.borderRadius,
    }
  })
  expect(dialogStyle).toEqual({
    width: page.viewportSize()!.width < 720 ? page.viewportSize()!.width - 28 : 480,
    paddingTop: page.viewportSize()!.width < 720 ? '20px' : '24px',
    paddingRight: page.viewportSize()!.width < 720 ? '16px' : '24px',
    radius: '20px',
  })
  const doneButton = dialog.getByRole('button', { name: /Done|完成/ })
  const doneColors = await doneButton.evaluate((element) => {
    const style = getComputedStyle(element)
    return { background: style.backgroundColor, color: style.color }
  })
  expect(doneColors).toEqual({ background: 'rgb(26, 26, 26)', color: 'rgb(255, 255, 255)' })
  await expect(dialog.getByRole('switch')).toHaveCount(7)
  const firstSwitch = dialog.getByRole('switch').first()
  await expect(firstSwitch).toHaveAttribute('aria-checked', 'true')
  await firstSwitch.click()
  await expect(firstSwitch).toHaveAttribute('aria-checked', 'false')
})

test('connection rows use the OpenAPI fields and persist search in the URL', async ({ page }) => {
  await page.goto('/connections')
  await page.waitForLoadState('networkidle')
  await expect(page.getByText('google.com:443').first()).toBeVisible()
  const search = page.getByPlaceholder(/Search destination|搜索目标/)
  await search.fill('google')
  await expect(page).toHaveURL(/q=google/, { timeout: 2000 })
  await page.getByText('google.com:443').first().click()
  if (page.viewportSize()!.width < 720) {
    await expect(page.getByRole('heading', { level: 1, name: /Details|详情/ })).toBeVisible()
    const back = page.getByRole('button', { name: /Connections|连接/ })
    await expect(back).toBeVisible()
    expect(await back.evaluate((element) => getComputedStyle(element).borderRadius)).toBe('8px')
  } else {
    const drawer = page.getByRole('dialog')
    await expect(drawer).toBeVisible()
    const drawerStyle = await drawer.evaluate((element) => {
      const rect = element.getBoundingClientRect()
      return {
        width: rect.width,
        right: window.innerWidth - rect.right,
        radius: getComputedStyle(element).borderRadius,
        buttons: element.querySelectorAll('button').length,
      }
    })
    expect(drawerStyle).toEqual({ width: 420, right: 0, radius: '0px', buttons: 0 })
  }
})

test('theme can be switched from settings', async ({ page }) => {
  await page.goto('/settings')
  await page.waitForLoadState('networkidle')
  await page.getByRole('combobox').first().click()
  await page.getByRole('option', { name: /Dark|深色/ }).click()
  await expect(page.locator('html')).toHaveClass(/dark/)
})
