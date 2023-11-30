import { shouldPolyfill as shouldPolyfillCanonicalLocales } from "@formatjs/intl-getcanonicallocales/should-polyfill";
import { shouldPolyfill as shouldPolyfillLocale } from "@formatjs/intl-locale/should-polyfill";
import { shouldPolyfill as shouldPolyfillNumberformat } from "@formatjs/intl-numberformat/should-polyfill";
import { shouldPolyfill as shouldPolyfillPluralRules } from "@formatjs/intl-pluralrules/should-polyfill";
import "intersection-observer";

async function checkPolyfills() {
  if (shouldPolyfillCanonicalLocales()) {
    await import("@formatjs/intl-getcanonicallocales/polyfill");
  }
  if (shouldPolyfillLocale()) {
    await import("@formatjs/intl-locale/polyfill");
  }
  if (shouldPolyfillNumberformat()) {
    await import("@formatjs/intl-numberformat/polyfill");
    await import("@formatjs/intl-numberformat/locale-data/en");
    await import("@formatjs/intl-numberformat/locale-data/en-GB");
  }
  if (shouldPolyfillPluralRules()) {
    await import("@formatjs/intl-pluralrules/polyfill");
    await import("@formatjs/intl-pluralrules/locale-data/en");
  }

  if (typeof window.ResizeObserver === "undefined") {
    const ResizeObserver = await import("resize-observer-polyfill");
    window.ResizeObserver = ResizeObserver.default;
  }
}

checkPolyfills();
