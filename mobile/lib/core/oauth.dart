/// Pure helpers for the Immich OAuth/SSO handoff.
///
/// Flow (mirrors the web app's login handling):
///   POST /api/auth/oauth/start  -> { url, state }   (Immich authorize URL)
///   user authenticates at the IdP
///   IdP -> GET/POST /api/auth/oauth/callback -> 302 `<swipe>/login?oauthCode=<code>`
///       or, for the mobile client, `<swipe>/oauth/mobile?code=<code>`
///   app -> POST /api/auth/oauth/finish { code } -> session
///
/// The mobile handoff page never consumes the code and is a plain HTML page, so
/// the in-app WebView can safely stop the navigation and finish the exchange
/// itself. All URL parsing is host-scoped: a code is only accepted from the
/// swipe backend the flow was started against, never from a redirect target
/// controlled by the identity provider.
library;

/// Normalised `scheme://host[:port]` for [url], or null when it has no origin.
String? _originOf(String url) {
  final uri = Uri.tryParse(url);
  if (uri == null || uri.host.isEmpty) return null;
  final scheme = uri.scheme.toLowerCase();
  if (scheme != 'http' && scheme != 'https') return null;
  final defaultPort = scheme == 'https' ? 443 : 80;
  final port = uri.hasPort && uri.port != defaultPort ? ':${uri.port}' : '';
  return '$scheme://${uri.host.toLowerCase()}$port';
}

/// Whether [url] points at the expected swipe backend.
///
/// When [expectedOrigin] is provided, both the scheme and host/port must match.
/// This prevents an IdP (or a malicious page) from handing the WebView a code
/// URL on a different host that the app would otherwise trust.
bool _matchesOrigin(String url, String? expectedOrigin) {
  if (expectedOrigin == null || expectedOrigin.trim().isEmpty) return true;
  final expected = _originOf(expectedOrigin.trim());
  if (expected == null) return false;
  return _originOf(url) == expected;
}

String? _nonEmpty(String? value) {
  final v = value?.trim();
  return (v == null || v.isEmpty) ? null : v;
}

/// `/oauth/mobile`, also when the backend is mounted under a path prefix
/// (`X-Forwarded-Prefix` / `SWIPE_PUBLIC_URL` with a path).
bool _isMobileHandoffPath(Uri uri) =>
    uri.path == '/oauth/mobile' || uri.path.endsWith('/oauth/mobile');

/// The one-time handoff code from the mobile `/oauth/mobile?code=...` page.
///
/// Only the exact `/oauth/mobile` path is accepted, and [expectedOrigin] (the
/// swipe backend base URL) must match when supplied.
String? oauthMobileCodeFromUrl(String url, {String? expectedOrigin}) {
  final uri = Uri.tryParse(url);
  if (uri == null || !_isMobileHandoffPath(uri)) return null;
  if (!_matchesOrigin(url, expectedOrigin)) return null;
  return _nonEmpty(uri.queryParameters['code']);
}

/// The error reported by the backend/IdP on the mobile handoff page.
String? oauthMobileErrorFromUrl(String url, {String? expectedOrigin}) {
  final uri = Uri.tryParse(url);
  if (uri == null || !_isMobileHandoffPath(uri)) return null;
  if (!_matchesOrigin(url, expectedOrigin)) return null;
  return _nonEmpty(uri.queryParameters['oauthError'] ?? uri.queryParameters['error']);
}

/// Human-readable description from the mobile handoff page, if provided.
String? oauthMobileErrorDescriptionFromUrl(String url, {String? expectedOrigin}) {
  final uri = Uri.tryParse(url);
  if (uri == null || !_isMobileHandoffPath(uri)) return null;
  if (!_matchesOrigin(url, expectedOrigin)) return null;
  return _nonEmpty(uri.queryParameters['oauthErrorDescription'] ??
      uri.queryParameters['error_description']);
}

/// The one-time handoff code from the browser SPA redirect
/// (`/login?oauthCode=...`), or a mobile handoff URL.
///
/// Retained for the manual-code fallback and backwards compatibility.
String? oauthCodeFromUrl(String url, {String? expectedOrigin}) {
  final uri = Uri.tryParse(url);
  if (uri == null) return null;
  if (!_matchesOrigin(url, expectedOrigin)) return null;
  final mobile = oauthMobileCodeFromUrl(url, expectedOrigin: expectedOrigin);
  if (mobile != null) return mobile;
  return _nonEmpty(uri.queryParameters['oauthCode']);
}

/// The error reported by the backend or the IdP on the redirect URL, if any.
String? oauthErrorFromUrl(String url, {String? expectedOrigin}) {
  final uri = Uri.tryParse(url);
  if (uri == null) return null;
  if (!_matchesOrigin(url, expectedOrigin)) return null;
  final mobile = oauthMobileErrorFromUrl(url, expectedOrigin: expectedOrigin);
  if (mobile != null) return mobile;
  return _nonEmpty(uri.queryParameters['oauthError'] ?? uri.queryParameters['error']);
}
