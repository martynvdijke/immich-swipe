import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:webview_flutter/webview_flutter.dart';

import '../../core/oauth.dart';

/// Fetches a brand-new authorize URL for a retried SSO flow.
///
/// A retry must call `POST /api/auth/oauth/start` again: replaying the original
/// authorize URL reuses a state/PKCE pair that the previous attempt may already
/// have consumed (or that expired), which is why reloading it often fails with
/// an opaque IdP error.
typedef OAuthStartUrlProvider = Future<String> Function();

/// Runs the Immich OAuth/SSO handoff inside an in-app browser.
///
/// The backend is started with `client=mobile`, so after the user authenticates
/// the IdP redirects to `<swipe>/oauth/mobile?code=<one-time-code>` (or
/// `?oauthError=...`). That navigation is intercepted and never loaded: the
/// screen pops with the code for `POST /api/auth/oauth/finish`.
///
/// Interception is deliberately form_post-safe:
///   * the IdP may answer with a POST to `/api/auth/oauth/callback` and a
///     303 redirect to `/oauth/mobile`;
///   * WebView navigation callbacks differ between GET redirects and committed
///     POST navigations, so the same check runs in `onNavigationRequest`,
///     `onUrlChange` and `onPageStarted`;
///   * only a `/oauth/mobile` handoff on [expectedOrigin] (the swipe backend the
///     flow was started against) is trusted, never a redirect to another host.
///
/// Pops with the code on success, or null when the user backs out.
class OAuthWebViewScreen extends StatefulWidget {
  const OAuthWebViewScreen({
    super.key,
    required this.startUrl,
    required this.buttonText,
    this.expectedOrigin,
    this.onRetry,
  });

  final String startUrl;
  final String buttonText;

  /// The swipe backend origin (e.g. `https://swipe.example.com`). Handoff URLs
  /// on any other host are ignored.
  final String? expectedOrigin;

  /// Starts a completely new OAuth flow for the Retry action.
  final OAuthStartUrlProvider? onRetry;

  @override
  State<OAuthWebViewScreen> createState() => _OAuthWebViewScreenState();
}

class _OAuthWebViewScreenState extends State<OAuthWebViewScreen> {
  late final WebViewController _controller;
  late String _startUrl;
  bool _loading = true;
  bool _retrying = false;
  String? _error;
  bool _handoffDone = false;

  @override
  void initState() {
    super.initState();
    _startUrl = widget.startUrl;
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setNavigationDelegate(
        NavigationDelegate(
          onUrlChange: (change) {
            final url = change.url;
            if (url != null) _handleUrl(url);
          },
          onPageStarted: (url) {
            // Fallback: a redirect may already have been committed before
            // onNavigationRequest could stop it.
            _handleUrl(url);
          },
          onPageFinished: (url) {
            _handleUrl(url);
            if (mounted && !_handoffDone) setState(() => _loading = false);
          },
          onWebResourceError: (e) {
            // Sub-resource failures (ads, images) are not worth a full error UI.
            if (e.isForMainFrame == false) return;
            if (mounted) {
              setState(() {
                _loading = false;
                _error = e.description;
              });
            }
          },
          onNavigationRequest: (req) {
            if (_handleUrl(req.url)) return NavigationDecision.prevent;
            return NavigationDecision.navigate;
          },
        ),
      );
    // A malformed/missing authorize URL must not crash the screen.
    try {
      _controller.loadRequest(Uri.parse(widget.startUrl));
    } catch (e) {
      _loading = false;
      _error = 'The server returned an invalid sign-in URL: $e';
    }
  }

  /// Checks one URL for a trusted mobile handoff.
  ///
  /// Returns true when the URL was consumed (success or error) and the WebView
  /// must not load it.
  bool _handleUrl(String url) {
    if (_handoffDone) return true;

    final code = oauthMobileCodeFromUrl(
      url,
      expectedOrigin: widget.expectedOrigin,
    );
    if (code != null) {
      _complete(code);
      return true;
    }

    final err = oauthMobileErrorFromUrl(
      url,
      expectedOrigin: widget.expectedOrigin,
    );
    if (err != null) {
      final description = oauthMobileErrorDescriptionFromUrl(
        url,
        expectedOrigin: widget.expectedOrigin,
      );
      _fail(
        description == null || description.isEmpty
            ? err
            : '$err — $description',
      );
      return true;
    }

    // Legacy browser SPA redirect, still accepted when host-scoped. Keeps the
    // manual fallback and older backends working.
    final legacy = oauthCodeFromUrl(url, expectedOrigin: widget.expectedOrigin);
    if (legacy != null && !url.contains('/oauth/mobile')) {
      _complete(legacy);
      return true;
    }
    return false;
  }

  void _fail(String message) {
    if (_handoffDone) return;
    _handoffDone = true;
    if (mounted) {
      setState(() {
        _loading = false;
        _error = message;
      });
    }
  }

  void _complete(String code) {
    if (_handoffDone) return;
    _handoffDone = true;
    // Popping from inside a navigation callback upsets the platform view, so
    // hand the code back on the next frame.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) Navigator.of(context).pop(code);
    });
  }

  Future<void> _retry() async {
    if (_retrying) return;
    setState(() {
      _retrying = true;
      _error = null;
      _loading = true;
      _handoffDone = false;
    });
    try {
      var url = _startUrl;
      if (widget.onRetry != null) {
        url = await widget.onRetry!();
      }
      if (!mounted) return;
      if (url.trim().isEmpty) {
        throw StateError('The server returned no sign-in URL');
      }
      _startUrl = url.trim();
      await _controller.loadRequest(Uri.parse(_startUrl));
    } catch (e) {
      if (mounted) {
        setState(() {
          _loading = false;
          _error = 'Could not restart sign-in: $e';
        });
      }
    } finally {
      if (mounted) setState(() => _retrying = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      appBar: AppBar(
        title: Text('Sign in with ${widget.buttonText}'),
        actions: [
          IconButton(
            tooltip: 'Copy SSO URL',
            icon: const Icon(Icons.link_rounded),
            onPressed: () async {
              await Clipboard.setData(ClipboardData(text: _startUrl));
              if (!context.mounted) return;
              ScaffoldMessenger.of(context).showSnackBar(
                const SnackBar(
                  content: Text(
                    'SSO URL copied — complete it in a browser and paste the code',
                  ),
                ),
              );
            },
          ),
        ],
        bottom: _loading
            ? const PreferredSize(
                preferredSize: Size.fromHeight(2),
                child: LinearProgressIndicator(),
              )
            : null,
      ),
      body: _error != null
          ? Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Icon(
                      Icons.error_outline_rounded,
                      size: 40,
                      color: scheme.error,
                    ),
                    const SizedBox(height: 12),
                    Text(
                      'SSO sign-in failed',
                      style: Theme.of(context).textTheme.titleMedium,
                    ),
                    const SizedBox(height: 6),
                    Text(
                      _error!,
                      textAlign: TextAlign.center,
                      style: Theme.of(context).textTheme.bodySmall,
                    ),
                    const SizedBox(height: 16),
                    Row(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        OutlinedButton(
                          onPressed: _retrying ? null : _retry,
                          child: _retrying
                              ? const SizedBox(
                                  width: 16,
                                  height: 16,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                  ),
                                )
                              : const Text('Retry'),
                        ),
                        const SizedBox(width: 12),
                        FilledButton(
                          onPressed: () => Navigator.of(context).pop(),
                          child: const Text('Enter code manually'),
                        ),
                      ],
                    ),
                  ],
                ),
              ),
            )
          : WebViewWidget(controller: _controller),
    );
  }
}
