/**
 * Cloudflare Worker — GitHub OAuth token exchange proxy.
 *
 * Receives an authorization code from the browser, exchanges it with GitHub
 * for an access token using the stored client_secret, and returns the token.
 *
 * Environment variables (set via `wrangler secret put`):
 *   GITHUB_CLIENT_ID     — GitHub OAuth App client ID
 *   GITHUB_CLIENT_SECRET — GitHub OAuth App client secret
 *
 * Deploy: wrangler deploy
 */

const CORS_HEADERS = {
  'Access-Control-Allow-Origin': 'https://graph.pm',
  'Access-Control-Allow-Methods': 'POST, OPTIONS',
  'Access-Control-Allow-Headers': 'Content-Type',
};

export default {
  async fetch(request, env) {
    // Handle CORS preflight
    if (request.method === 'OPTIONS') {
      return new Response(null, { status: 204, headers: CORS_HEADERS });
    }

    if (request.method !== 'POST') {
      return new Response('Method not allowed', { status: 405, headers: CORS_HEADERS });
    }

    try {
      const { code } = await request.json();
      if (!code) {
        return Response.json({ error: 'Missing code' }, { status: 400, headers: CORS_HEADERS });
      }

      const resp = await fetch('https://github.com/login/oauth/access_token', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'application/json',
        },
        body: JSON.stringify({
          client_id: env.GITHUB_CLIENT_ID,
          client_secret: env.GITHUB_CLIENT_SECRET,
          code,
        }),
      });

      const data = await resp.json();

      if (data.error) {
        return Response.json({ error: data.error_description || data.error }, {
          status: 400,
          headers: CORS_HEADERS,
        });
      }

      return Response.json({ access_token: data.access_token }, { headers: CORS_HEADERS });
    } catch (err) {
      return Response.json({ error: 'Internal error' }, { status: 500, headers: CORS_HEADERS });
    }
  },
};
