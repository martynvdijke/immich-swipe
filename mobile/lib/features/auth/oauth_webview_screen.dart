import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:webview_flutter/webview_flutter.dart';
import '../../core/oauth.dart';

/// Runs the Immich OAuth/SSO handoff inside an in-app browser.
///
/// [startUrl] is the Immich authorize URL returned by `POST /api/auth/oauth/start`.
/// After the user authenticates, the IdP redirects to the swipe backend's
/// `/api/auth/oauth/callback`, which answers `302 <swipe>/login?oauthCode=<code>`.
/// That navigation is intercepted (never loaded: it is the SPA route), and the
/// screen pops with the one-time code for `POST /api/auth/oauth/finish`.
///
/// Pops with the code on success, or null when the user backs out.
class OAuthWebViewScreen extends StatefulWidget {
  const OAuthWebViewScreen({super.key, required this.startUrl, required this.buttonText});

  final String startUrl;
  final String buttonText;

  @override
  State<OAuthWebViewScreen> createState() => _OAuthWebViewScreenState();
}

class _OAuthWebViewScreenState extends State<OAuthWebViewScreen> {
  late final WebViewController _controller;
  bool _loading = true;
  String? _error;
  bool _handoffDone = false;

  @override
  void initState() {
    super.initState();
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setNavigationDelegate(NavigationDelegate(
        onPageStarted: (url) {
          // Fallback: a redirect may already have been committed before
          // onNavigationRequest could stop it.
          final code = oauthCodeFromUrl(url);
          if (code != null) _complete(code);
        },
        onPageFinished: (url) {
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
          final code = oauthCodeFromUrl(req.url);
          if (code != null) {
            _complete(code);
            return NavigationDecision.prevent;
          }
          final err = oauthErrorFromUrl(req.url);
          if (err != null) {
            if (mounted) {
              setState(() {
                _loading = false;
                _error = err;
              });
            }
            return NavigationDecision.prevent;
          }
          return NavigationDecision.navigate;
        },
      ))
      ..loadRequest(Uri.parse(widget.startUrl));
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

  Future<void> _reload() async {
    setState(() {
      _error = null;
      _loading = true;
      _handoffDone = false;
    });
    await _controller.loadRequest(Uri.parse(widget.startUrl));
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
              await Clipboard.setData(ClipboardData(text: widget.startUrl));
              if (!context.mounted) return;
              ScaffoldMessenger.of(context).showSnackBar(
                  const SnackBar(content: Text('SSO URL copied — complete it in a browser and paste the code')));
            },
          ),
        ],
        bottom: _loading ? const PreferredSize(preferredSize: Size.fromHeight(2), child: LinearProgressIndicator()) : null,
      ),
      body: _error != null
          ? Center(
              child: Padding(
                padding: const EdgeInsets.all(24),
                child: Column(mainAxisSize: MainAxisSize.min, children: [
                  Icon(Icons.error_outline_rounded, size: 40, color: scheme.error),
                  const SizedBox(height: 12),
                  Text('SSO sign-in failed', style: Theme.of(context).textTheme.titleMedium),
                  const SizedBox(height: 6),
                  Text(_error!, textAlign: TextAlign.center, style: Theme.of(context).textTheme.bodySmall),
                  const SizedBox(height: 16),
                  Row(mainAxisAlignment: MainAxisAlignment.center, children: [
                    OutlinedButton(onPressed: _reload, child: const Text('Retry')),
                    const SizedBox(width: 12),
                    FilledButton(
                        onPressed: () => Navigator.of(context).pop(),
                        child: const Text('Enter code manually')),
                  ]),
                ]),
              ),
            )
          : WebViewWidget(controller: _controller),
    );
  }
}
