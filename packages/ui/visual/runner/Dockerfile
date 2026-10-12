# Visual regression runner: Chromium, fonts and Playwright pinned together so baselines are
# identical on every machine and in CI. The Playwright image version must equal @playwright/test.
FROM mcr.microsoft.com/playwright:v1.63.0-noble@sha256:eff16c30e6f3f4af0a03fa4b706120d5e9b0891c344a27d64559aff5900a4a27
WORKDIR /runner
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund --ignore-scripts && npm cache clean --force
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 CI=1
