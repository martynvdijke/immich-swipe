/// Pure helpers for the Immich OAuth/SSO handoff.
///
/// Flow (mirrors the web app's LoginView query handling):
///   POST /api/auth/oauth/start  -> { url, state }   (Immich authorize URL)
///   user authenticates at the IdP
///   IdP -> GET /api/auth/oauth/callback -> 302 `<swipe>/login?oauthCode=<code>`
///   app -> POST /api/auth/oauth/finish { code } -> session
///
/// The redirect target is the swipe web app's own `/login` route, so the app
/// intercepts that navigation instead of loading the SPA.
library;

/// The one-time handoff code from the backend redirect URL, or null when [url]
/// is not the redirect target.
String? oauthCodeFromUrl(String url) {
  final uri = Uri.tryParse(url);
  if (uri == null) return null;
  final code = uri.queryParameters['oauthCode'];
  return (code == null || code.isEmpty) ? null : code;
}

/// The error reported by the backend or the IdP on the redirect URL, if any.
String? oauthErrorFromUrl(String url) {
  final uri = Uri.tryParse(url);
  if (uri == null) return null;
  final err = uri.queryParameters['oauthError'] ?? uri.queryParameters['error'];
  return (err == null || err.isEmpty) ? null : err;
}
