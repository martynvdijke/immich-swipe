/// Builds a public legal-page URL on the swipe backend.
///
/// The pages are served by the Go backend (`/privacy`, `/terms`,
/// `/account/delete`) and are readable without a session, so the in-app browser
/// can load them directly.
String legalUrlFor(String serverUrl, String path) {
  final base = serverUrl.trim().replaceAll(RegExp(r'/+$'), '');
  final suffix = path.startsWith('/') ? path : '/$path';
  return '$base$suffix';
}
