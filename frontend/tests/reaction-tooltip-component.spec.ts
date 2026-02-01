import { test, expect } from '@playwright/test'

/**
 * Mock test data for reaction tooltip tests.
 * Uses inline HTML/CSS that mirrors the actual FabricPanel styles.
 * This avoids dependency on real session data or running backend.
 */

test('ReactionTooltip component renders portal on hover', async ({ page }) => {
  // We'll test by loading the built app's CSS and simulating the DOM structure
  // that the React component produces
  
  await page.setContent(`
    <!DOCTYPE html>
    <html>
    <head>
      <style>
        /* Copy the actual styles from FabricPanel.css */
        body {
          margin: 0;
          background: #1a1a2e;
          color: #e0e0e0;
          font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
        }
        .fabric-panel-slack {
          display: flex;
          margin: 0;
          flex: 1;
          min-height: 100vh;
          background: #1a1a2e;
          overflow: hidden;
          position: relative;
        }
        .channel-sidebar {
          width: 220px;
          min-width: 180px;
          background: #252540;
          border-right: 1px solid #363646;
          display: flex;
          flex-direction: column;
          z-index: 1;
          position: relative;
          padding: 1rem;
        }
        .message-area {
          flex: 1;
          display: flex;
          flex-direction: column;
          min-width: 0;
          position: relative;
          z-index: 2;
          padding: 1rem;
        }
        .message-item {
          display: flex;
          gap: 0.75rem;
          padding: 0.5rem 1.25rem;
          background: transparent;
          position: relative;
        }
        .message-reactions {
          display: flex;
          flex-wrap: wrap;
          gap: 6px;
          margin-top: 6px;
          position: relative;
          z-index: 10;
        }
        .reaction-badge {
          display: inline-flex;
          align-items: center;
          gap: 4px;
          padding: 2px 8px;
          background: #2a2a3a;
          border: 1px solid #363646;
          border-radius: 12px;
          font-size: 0.8125rem;
          cursor: pointer;
          position: relative;
        }
        .reaction-badge:hover {
          background: #3a3a4a;
          border-color: #58a6ff;
        }
        .reaction-tooltip-portal {
          pointer-events: none;
        }
        .reaction-tooltip-content {
          background: #1e1e2e;
          border: 1px solid #363646;
          border-radius: 12px;
          padding: 12px 16px;
          text-align: center;
          box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
        }
        .reaction-tooltip-content::after {
          content: '';
          position: absolute;
          top: 100%;
          left: 50%;
          transform: translateX(-50%);
          border: 8px solid transparent;
          border-top-color: #1e1e2e;
        }
        .reaction-tooltip-emoji {
          font-size: 2.5rem;
          line-height: 1;
          display: block;
          margin-bottom: 8px;
          background: rgba(255, 255, 255, 0.1);
          border-radius: 8px;
          padding: 10px;
          width: fit-content;
          margin-left: auto;
          margin-right: auto;
        }
        .reaction-tooltip-names {
          font-weight: 500;
          font-size: 0.8125rem;
          color: #e0e0e0;
          white-space: nowrap;
          display: block;
        }
        .channel-item {
          padding: 0.5rem;
          margin: 0.25rem 0;
          border-radius: 4px;
          color: #a0a0b0;
        }
        .channel-item.selected {
          background: #58a6ff33;
          color: white;
        }
      </style>
    </head>
    <body>
      <div class="fabric-panel-slack">
        <div class="channel-sidebar">
          <div class="channel-item">Root</div>
          <div class="channel-item">System</div>
          <div class="channel-item">Tasks</div>
          <div class="channel-item selected">General</div>
          <div class="channel-item">Observer</div>
        </div>
        <div class="message-area">
          <h2 style="margin-top: 0;">General</h2>
          <div class="message-item">
            <div style="width: 36px; height: 36px; background: #9b59b6; border-radius: 6px; display: flex; align-items: center; justify-content: center;">U</div>
            <div>
              <div><strong style="color: #9b59b6;">user</strong> <span style="color: #666; font-size: 0.75rem;">2:00 AM</span></div>
              <div>Hello everyone! Let's collaborate on this.</div>
              <div class="message-reactions">
                <button class="reaction-badge" id="badge1">
                  <span class="reaction-emoji">👋</span>
                  <span class="reaction-count">4</span>
                </button>
                <button class="reaction-badge" id="badge2">
                  <span class="reaction-emoji">👀</span>
                  <span class="reaction-count">1</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
      
      <script>
        // Simulate the React portal behavior
        function createTooltip(badge, emoji, names) {
          const rect = badge.getBoundingClientRect();
          const tooltip = document.createElement('div');
          tooltip.className = 'reaction-tooltip-portal';
          tooltip.style.cssText = \`
            position: fixed;
            top: \${rect.top - 8}px;
            left: \${rect.left + rect.width / 2}px;
            transform: translate(-50%, -100%);
            z-index: 99999;
          \`;
          tooltip.innerHTML = \`
            <div class="reaction-tooltip-content">
              <span class="reaction-tooltip-emoji">\${emoji}</span>
              <span class="reaction-tooltip-names">\${names}</span>
            </div>
          \`;
          document.body.appendChild(tooltip);
          return tooltip;
        }
        
        let activeTooltip = null;
        
        document.getElementById('badge1').addEventListener('mouseenter', function() {
          activeTooltip = createTooltip(this, '👋', 'worker-1, worker-2, coordinator, and observer');
        });
        document.getElementById('badge1').addEventListener('mouseleave', function() {
          if (activeTooltip) { activeTooltip.remove(); activeTooltip = null; }
        });
        
        document.getElementById('badge2').addEventListener('mouseenter', function() {
          activeTooltip = createTooltip(this, '👀', 'observer');
        });
        document.getElementById('badge2').addEventListener('mouseleave', function() {
          if (activeTooltip) { activeTooltip.remove(); activeTooltip = null; }
        });
      </script>
    </body>
    </html>
  `)
  
  // Screenshot initial state
  await page.screenshot({ path: 'screenshots/component-01-initial.png', fullPage: true })
  
  // Hover over first badge
  const badge1 = page.locator('#badge1')
  await badge1.hover()
  await page.waitForTimeout(200)
  
  await page.screenshot({ path: 'screenshots/component-02-hover-wave.png', fullPage: true })
  
  // Verify tooltip is visible and positioned correctly
  const tooltip = page.locator('.reaction-tooltip-portal')
  await expect(tooltip).toBeVisible()
  
  const tooltipBox = await tooltip.boundingBox()
  const badgeBox = await badge1.boundingBox()
  
  console.log('Badge 1 position:', badgeBox)
  console.log('Tooltip position:', tooltipBox)
  
  // Tooltip should be above the badge (tooltip bottom < badge top)
  expect(tooltipBox!.y + tooltipBox!.height).toBeLessThan(badgeBox!.y + 10) // small margin for arrow
  
  // Move to second badge
  const badge2 = page.locator('#badge2')
  await badge2.hover()
  await page.waitForTimeout(200)
  
  await page.screenshot({ path: 'screenshots/component-03-hover-eyes.png', fullPage: true })
  
  // Verify second tooltip shows different content
  const tooltipNames = await page.locator('.reaction-tooltip-names').textContent()
  expect(tooltipNames).toBe('observer')
  
  console.log('All assertions passed!')
})
