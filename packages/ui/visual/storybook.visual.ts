// Every Storybook story, rendered in the light and dark theme: compared with the committed
// baseline screenshots and checked with axe (WCAG 2.2 AA rules, including colour contrast).
// Stories tagged `novisual` are skipped; `vp-<width>` sets the viewport (shell stories).
import AxeBuilder from '@axe-core/playwright';
import { expect, test } from '@playwright/test';
import { readFileSync } from 'node:fs';

interface IndexEntry {
  id: string;
  type: 'story' | 'docs';
  tags?: string[];
}

const index = JSON.parse(readFileSync('/storybook/index.json', 'utf8')) as {
  entries: Record<string, IndexEntry>;
};
const stories = Object.values(index.entries).filter(
  (entry) => entry.type === 'story' && !entry.tags?.includes('novisual'),
);

for (const story of stories) {
  for (const theme of ['light', 'dark'] as const) {
    test(`${story.id} · ${theme}`, async ({ page }) => {
      const viewportTag = story.tags?.find((tag) => tag.startsWith('vp-'));
      const width = viewportTag ? Number(viewportTag.slice(3)) : undefined;
      if (width) await page.setViewportSize({ width, height: width <= 760 ? 844 : 900 });

      await page.goto(`/iframe.html?id=${story.id}&viewMode=story&globals=theme:${theme}`);
      await page.locator('#storybook-root > *').first().waitFor({ state: 'attached' });
      await page.evaluate(() => document.fonts.ready);
      await page.waitForTimeout(300); // popover positioning and play functions settle

      await expect(page).toHaveScreenshot(`${story.id}--${theme}.png`, { fullPage: !width });

      const results = await new AxeBuilder({ page })
        .withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa'])
        .analyze();
      expect(
        results.violations.map(
          (v) => `${v.id}: ${v.help} — ${v.nodes.map((n) => n.target.join(' ')).join(', ')}`,
        ),
      ).toEqual([]);
    });
  }
}
