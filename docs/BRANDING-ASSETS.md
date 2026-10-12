# MateCollab Branding Assets

Status: Phase 0 inventory · Source of truth: `MateCollab-Logo-Library/`

> **Authority rule.** `MateCollab-Logo-Library` is the only approved source of MateCollab logos and brand colours. Where the MateCollab-UI prototype differs, **the Library wins**. Production code must never redraw, re-colour, re-space or approximate the mark or wordmark.

---

## 1. Brand essentials

| Element | Value | Source |
|---|---|---|
| Primary Violet | `#7047D1` | Library README, all violet assets |
| Navy | `#101A2D` | Library README, dark assets |
| White | `#FFFFFF` | Library README |
| Mark geometry | Four equal sharp L-shaped corners and a centred square on a 1000 × 1000 grid (arms 80 units thick, corner legs 320 units, centre square 200 units) | `favicons/favicon.svg` |
| Wordmark | "MateCollab" in **HemiHead Bold**, supplied only as outlined vector paths inside the lockups | `lockups/*.svg` |
| Selected direction | "C, Violet" | Library README |

Usage rules from the Library README (normative):

* The mark always keeps **sharp corners**. Rounded/circular *backgrounds* are allowed; rounding the mark is not.
* Keep clear space of at least **one arm thickness** (8 % of the icon size) around the visible mark.
* Do not stretch, rotate individual parts, round the geometry, or add shadows.
* Use the **Violet icon on white**, and the **white icon on Navy or Violet** for highest clarity. "Dark violet" (violet mark on navy) is decorative only; prefer white for very small icons on dark UI.
* **HemiHead is for branding only.** Application UI uses a normal readable sans-serif (Inter — see [FRONTEND.md](architecture/FRONTEND.md#6-typography-and-brand)).
* Square app-icon exports include 10 % padding; they are standard (not maskable) icons.

---

## 2. Library location

```
MateCollab/
└─ MateCollab-Logo-Library/            ← top folder (untracked in git today)
   └─ MateCollab-Logo-Library/         ← actual library root (double-nested)
      ├─ README.md
      ├─ MateCollab-Violet-Preview.jpg
      ├─ favicons/   (9 files)
      ├─ icons/      (15 variant folders, 276 files)
      └─ lockups/    (33 files)
```

The double nesting is preserved as delivered. Production code will reference assets through a single constant path (`LOGO_LIBRARY_ROOT = MateCollab-Logo-Library/MateCollab-Logo-Library`) so the folder can be flattened later — by the brand owner, not by engineering — without touching application code.

---

## 3. Inventory

**Totals:** 320 files — 246 PNG, 46 JPG, 22 SVG, 4 ICO, 1 web manifest, 1 README. **No font files.**

### 3.1 Icons (`icons/<variant>/`)

Each background variant contains one SVG, 15 PNG sizes (16, 20, 24, 32, 48, 64, 96, 128, 180, 192, 256, 384, 512, 1024, 2048 px) and 3 JPG sizes (512, 1024, 2048 px). Transparent variants have SVG + 15 PNG (no JPG, since JPG cannot store transparency).

| Variant | Background | Mark | Shape | Files | Primary use |
|---|---|---|---|---|---|
| `transparent-violet` | none | Violet | — | 16 | **Primary icon.** Light UI surfaces, favicon source |
| `transparent-white` | none | White | — | 16 | **Dark UI**: navigation rail, dark headers, meeting stage |
| `transparent-black` | none | Black | — | 16 | Monochrome print / single-colour contexts |
| `square-light` | White | Violet | square | 19 | Light tiles, documents |
| `square-dark` | Navy | White | square | 19 | Dark tiles |
| `square-violet` | Violet | White | square | 19 | **App icon** (used by the favicon set) |
| `square-dark-violet` | Navy | Violet | square | 19 | Decorative only |
| `rounded-light` | White | Violet | rounded (r = 18 %) | 19 | Light avatar/app tiles |
| `rounded-dark` | Navy | White | rounded | 19 | Dark avatar/app tiles |
| `rounded-violet` | Violet | White | rounded | 19 | App tiles and marketing; alternative system/bot avatar (256/512 PNG). Not used for organisation avatars, which show the organisation initial |
| `rounded-dark-violet` | Navy | Violet | rounded | 19 | Decorative only |
| `circle-light` | White | Violet | circle | 19 | Circular avatar slots on light UI |
| `circle-dark` | Navy | White | circle | 19 | Circular avatar slots on dark UI |
| `circle-violet` | Violet | White | circle | 19 | System user avatar ("MateCollab" bot / system messages) |
| `circle-dark-violet` | Navy | Violet | circle | 19 | Decorative only |

### 3.2 Horizontal lockups (`lockups/`)

Icon + HemiHead wordmark, `viewBox 0 0 1670 320`. SVG plus PNG at 1024, 2048 and 4096 px wide; opaque treatments also have JPG at the same widths.

| File stem | Background | Mark | Wordmark | Files | Use |
|---|---|---|---|---|---|
| `MateCollab-transparent` | none | Violet | Navy | 4 | **Light theme** headers, login, documents |
| `MateCollab-white` | none | White | White | 4 | **Dark theme**, navy rail areas, violet panels |
| `MateCollab-light` | White | Violet | Navy | 7 | Opaque light placements (emails, slides) |
| `MateCollab-dark` | Navy | White | White | 7 | Opaque dark placements |
| `MateCollab-brand` | Violet | White | White | 7 | Marketing/hero, social images |
| `MateCollab-violet` | none | Violet | Violet | 4 | Single-colour violet contexts |

### 3.3 Favicons and app icons (`favicons/`)

| File | Details | Use |
|---|---|---|
| `favicon.svg` | Transparent violet mark | Modern browsers (`<link rel="icon" type="image/svg+xml">`) |
| `transparent-violet.ico` | 7 sizes: 16, 24, 32, 48, 64, 128, 256 | Legacy `favicon.ico` (recommended default) |
| `square-violet.ico`, `rounded-violet.ico`, `square-dark.ico` | Same 7 sizes | Alternatives |
| `apple-touch-icon.png` | 180 × 180, square violet, white mark | iOS home screen |
| `icon-192.png`, `icon-512.png` | Square violet, white mark | PWA manifest |
| `site.webmanifest` | `name/short_name: MateCollab`, `theme_color #7047D1`, `background_color #101A2D`, `display: standalone`, icons `purpose: any` | Web app manifest (paths must be rewritten when copied) |

### 3.4 Other

| File | Use |
|---|---|
| `MateCollab-Violet-Preview.jpg` | Visual reference of the identity (lockup, icon treatments, size ramp 16–64 px) |
| `README.md` | Usage rules (summarised in §1) |

---

## 4. Requested asset categories

| Category | Answer |
|---|---|
| Primary icon | `icons/transparent-violet/transparent-violet.svg` (identical geometry to `favicons/favicon.svg`) |
| Wordmark | Exists **only inside lockups** (outlined HemiHead Bold). There is no standalone wordmark file. |
| Light assets | `lockups/MateCollab-transparent.*`, `lockups/MateCollab-light.*`, `icons/transparent-violet`, `*-light` icon variants |
| Dark assets | `lockups/MateCollab-white.*`, `lockups/MateCollab-dark.*`, `icons/transparent-white`, `*-dark` icon variants |
| SVG assets | 22 (15 icons, 6 lockups, 1 favicon) |
| PNG assets | 246 (225 icon sizes, 18 lockup sizes, 3 favicon/app icons) |
| Favicon assets | `favicon.svg`, 4 × `.ico`, `apple-touch-icon.png`, `icon-192.png`, `icon-512.png`, `site.webmanifest` |
| Fonts | **None.** HemiHead is not included and is not needed (outlined in SVG). |

---

## 5. Production usage map

| Surface | Light theme | Dark theme | Notes |
|---|---|---|---|
| Navigation rail brand button (rail is Navy in both themes) | `transparent-white` SVG, 28 px | same | White on Navy per README; prototype used a blue mark here |
| Login / password / invite pages | `MateCollab-transparent.svg` | `MateCollab-white.svg` | Theme-aware `<BrandLockup>` component |
| Sidebar footer lockup | `MateCollab-transparent.svg` at ~20 px height | `MateCollab-white.svg` | Optional; keep at ≥ 18 px height for legibility |
| Meeting pre-join / stage header | `transparent-white` icon (stage is dark) | same | |
| Favicon | `favicon.svg` + `transparent-violet.ico` | same | Browser chrome handles theme |
| Apple touch / PWA | `apple-touch-icon.png`, `icon-192.png`, `icon-512.png` | same | Manifest copied with corrected paths |
| System user avatar | `circle-violet` 256 PNG/SVG | same | For system messages and bots |
| Default organisation avatar | Organisation initial (as prototype) — **not** the MateCollab mark | same | The MateCollab mark identifies the product, not tenants |
| Transactional emails | `lockups/MateCollab-light-1024.png` scaled to 200–240 px width | — | Emails cannot rely on SVG; host the PNG on the app origin |
| Social / Open Graph | `MateCollab-brand-1024.png` on a 1200 × 630 canvas | — | Composition requires brand-owner approval (see §7) |

### Implementation rules

* Assets are consumed by a **build-time sync script** (Phase 1) that copies a curated, listed subset from the Library into `apps/web/public/brand/` and the Next.js icon slots, verifying SHA-256 hashes against the Library. CI fails if a copied asset diverges from its Library source. The Library itself is never edited by tooling.
* SVGs are rendered as `<img>` or inlined **unchanged** (no colour overrides via CSS `fill`, no path edits). Theme switching selects a different Library file; it never recolours one.
* The `<BrandLockup>` and `<BrandMark>` components are the only way application code renders the brand, which keeps every usage auditable.
* Minimum rendered sizes: mark ≥ 16 px (validated in the Library preview); lockup ≥ 18 px tall.

---

## 6. Conflicts with the MateCollab-UI prototype

| Prototype | Library (wins) | Resolution |
|---|---|---|
| Mark: two forward-leaning forms (`public/brand/*.svg`, inline `Logo` component) | Four sharp L corners + centred square | Replace every prototype mark with Library assets |
| Primary blue `#2463EB` (buttons, links, focus, active states) | Violet `#7047D1` | Product accent becomes Violet (decision **ADR-008**, accepted — see [DECISIONS.md](architecture/DECISIONS.md)) |
| Navy `#152235` / rail `#142236` | Navy `#101A2D` | Rail and brand navy surfaces use `#101A2D` |
| Live-text wordmark rendered with `@font-face HemiHead` | Outlined lettering in SVG lockups | Use SVG lockups; do not serve the font |
| `HemiHead-Bold.otf` shipped in `public/fonts/` and base64-embedded in `MateCollab-Prototype.html` | No font in the Library | Do not distribute the font (see §8) |
| App icon concept: blue rounded square (rx 17 %) | `square-violet` / `rounded-violet` exports | Use Library favicons/app icons |
| "Identity board" route `#/brand` | Library preview JPG | Not shipped; brand documentation lives here |

---

## 7. Gaps — requests for the brand owner (no engineering workaround)

Engineering will not derive or invent these; each needs a supplied asset or explicit approval:

1. **Wordmark-only SVG** (no icon), if any surface needs the name without the mark.
2. **Maskable PWA icons** (Android adaptive icons need a larger safe zone than the current 10 % padding).
3. **Monochrome SVG for Safari pinned tabs** (optional; `transparent-black.svg` may be approved for this).
4. **Email-sized PNG lockups** (e.g. 240/480 px wide). Downscaling the 1024 px PNG in HTML is acceptable meanwhile.
5. **Open Graph / social card composition** (1200 × 630).

---

## 8. HemiHead licensing note

The prototype's `HemiHead-Bold.otf` reports: *Hemi Head Bold, Version 2.000, © 2010–2026 Typodermic Fonts Inc., designer Raymond Larabie*, with OS/2 `fsType = 4` (preview-and-print embedding). Serving the font file to browsers, or base64-embedding it, is a web-font use whose permission depends on the purchased licence. Production avoids the question entirely by using the Library's outlined lockups. If live HemiHead text is ever required, confirm the Typodermic licence covers web embedding first.
