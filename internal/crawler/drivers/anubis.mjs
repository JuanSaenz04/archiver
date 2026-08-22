const preparedOrigins = new Map();

async function prepareOrigin(page, targetUrl) {
  const preflight = new URL(targetUrl);
  preflight.searchParams.set("__bx_anubis_preflight", Date.now().toString());

  await page.goto(preflight.href, {
    waitUntil: ["load", "networkidle2"],
    timeout: 180_000,
  });

  await page.waitForFunction(
    () => !document.getElementById("anubis_challenge"),
    { polling: 250, timeout: 120_000 },
  );
}

export default async function ({ page, data, crawler, seed }) {
  const origin = new URL(data.url).origin;

  let preparation = preparedOrigins.get(origin);
  if (!preparation) {
    preparation = prepareOrigin(page, data.url);
    preparedOrigins.set(origin, preparation);
  }

  try {
    await preparation;
  } catch (error) {
    if (preparedOrigins.get(origin) === preparation) {
      preparedOrigins.delete(origin);
    }
    throw error;
  }

  await crawler.loadPage(page, data, seed);
}
