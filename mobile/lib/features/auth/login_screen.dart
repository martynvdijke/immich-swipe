// ignore_for_file: deprecated_member_use, use_build_context_synchronously
import 'dart:async';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../state/providers.dart';
import '../../core/api_client.dart';
import '../../core/app_config.dart';
import '../../models/session.dart';
import 'oauth_webview_screen.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});
  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _serverCtrl = TextEditingController();
  final _userCtrl = TextEditingController();
  final _passCtrl = TextEditingController();
  final _apiKeyCtrl = TextEditingController();
  final _oauthCodeCtrl = TextEditingController();
  bool _createMode = false;
  bool _busy = false;
  String? _error;
  bool _needsApiKey = false;
  bool _obscure = true;
  bool _probing = false;
  final _serverFocus = FocusNode();
  Timer? _ssoDebounce;
  bool _ssoChecked = false;

  @override
  void initState() {
    super.initState();
    // First run points at the hosted instance. A previously saved backend URL
    // replaces this once storage has been read (see _loadSavedServerUrl).
    _serverCtrl.text = kDefaultServerUrl;
    // OAuth availability comes from GET /api/auth/config, which is otherwise only
    // fetched after a session exists — so a fresh install would never surface the
    // SSO button. Probe the typed server after a short debounce while typing, and
    // immediately when the field loses focus.
    _serverCtrl.addListener(_onServerChanged);
    _serverFocus.addListener(() {
      if (!_serverFocus.hasFocus) {
        _ssoDebounce?.cancel();
        _checkOauthConfig();
      }
    });
    _loadSavedServerUrl();
    // Fresh installs already have the hosted default in the field. Probe it
    // immediately instead of waiting for the user to type or blur, otherwise
    // the SSO button/demo account would not appear on first launch.
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _checkOauthConfig();
    });
  }

  /// Loads the last successful backend URL. Storage wins over the hosted default
  /// but never over a URL the user has already typed.
  Future<void> _loadSavedServerUrl() async {
    final saved = await ref.read(secureStoreProvider).readLastServerUrl();
    if (!mounted || saved == null) return;
    if (_serverCtrl.text.trim() != kDefaultServerUrl) return;
    if (saved.trim() == kDefaultServerUrl) return;
    _serverCtrl.text = saved.trim();
  }

  void _onServerChanged() {
    _ssoDebounce?.cancel();
    if (!mounted) return;
    final url = _serverCtrl.text.trim();
    setState(() {
      _ssoChecked = false;
      _probing = url.isNotEmpty;
      ref.read(sessionProvider.notifier).clearOauthConfig();
    });
    if (url.isEmpty) return;
    _ssoDebounce = Timer(const Duration(milliseconds: 400), _checkOauthConfig);
  }

  Future<void> _checkOauthConfig() async {
    final url = _serverCtrl.text.trim();
    if (url.isEmpty) return;
    if (mounted) setState(() => _probing = true);
    final ok = await ref.read(sessionProvider.notifier).fetchConfigFor(url);
    if (!mounted) return;
    // Ignore a stale result if the field changed while the request was in flight.
    if (_serverCtrl.text.trim() != url) return;
    setState(() {
      _ssoChecked = ok;
      _probing = false;
    });
  }

  @override
  void dispose() {
    _ssoDebounce?.cancel();
    _serverFocus.dispose();
    _serverCtrl.dispose();
    _userCtrl.dispose();
    _passCtrl.dispose();
    _apiKeyCtrl.dispose();
    _oauthCodeCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    final url = _serverCtrl.text.trim();
    final user = _userCtrl.text.trim();
    final pass = _passCtrl.text;
    // The server URL is prefilled with the hosted instance but remains editable,
    // so validate before hitting the network instead of failing with a dio error.
    if (url.isEmpty || user.isEmpty || pass.isEmpty) {
      setState(() => _error = 'Server URL, user name and password are required');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      if (_createMode) {
        await ref.read(sessionProvider.notifier).createAccount(
            serverUrl: url,
            userName: user,
            password: pass,
            apiKey: _apiKeyCtrl.text.trim().isEmpty ? null : _apiKeyCtrl.text.trim());
      } else {
        await ref.read(sessionProvider.notifier).login(serverUrl: url, userName: user, password: pass);
        if (ref.read(sessionProvider).error?.contains('api_key_required') == true) {
          setState(() => _needsApiKey = true);
          return;
        }
      }
    } on AppException catch (e) {
      setState(() => _error = '${e.code}: ${e.message}');
      if (e.code == 'api_key_required') setState(() => _needsApiKey = true);
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _submitApiKey() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(sessionProvider.notifier).setApiKey(_apiKeyCtrl.text.trim());
      setState(() => _needsApiKey = false);
    } on AppException catch (e) {
      setState(() => _error = '${e.code}: ${e.message}');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Starts the SSO handoff: `POST /api/auth/oauth/start` returns the Immich
  /// authorize URL, which we run in an in-app browser. The backend callback
  /// redirects to `<swipe>/login?oauthCode=<code>`; the WebView screen intercepts
  /// that navigation and returns the one-time code, which we then exchange for a
  /// session via `POST /api/auth/oauth/finish`.
  Future<void> _oauthStart() async {
    final serverUrl = _serverCtrl.text.trim();
    if (serverUrl.isEmpty) {
      setState(() => _error = 'Enter your server URL first');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    String? code;
    try {
      final res = await ref.read(sessionProvider.notifier).oauthStart(serverUrl: serverUrl);
      final authorizeUrl = res['url'] as String? ?? '';
      if (authorizeUrl.isEmpty) {
        setState(() => _error = 'oauth_not_enabled: server returned no authorize URL');
        return;
      }
      if (!mounted) return;
      code = await Navigator.of(context).push<String>(MaterialPageRoute(
        builder: (_) => OAuthWebViewScreen(
          startUrl: authorizeUrl,
          buttonText: ref.read(sessionProvider).oauthConfig?.oauthButtonText ?? 'SSO',
          expectedOrigin: serverUrl,
          onRetry: () async {
            final fresh = await ref.read(sessionProvider.notifier).oauthStart(serverUrl: serverUrl);
            final freshUrl = fresh['url'] as String? ?? '';
            if (freshUrl.isEmpty) {
              throw AppException('oauth_not_enabled', 'Server returned no authorize URL');
            }
            return freshUrl;
          },
        ),
      ));
    } on AppException catch (e) {
      setState(() => _error = '${e.code}: ${e.message}');
    } catch (e) {
      setState(() => _error = e.toString());
    } finally {
      if (mounted) setState(() => _busy = false);
    }
    if (!mounted) return;
    // Backing out of the WebView leaves the manual paste path available.
    if (code == null || code.isEmpty) return;
    _oauthCodeCtrl.text = code;
    await _oauthFinish();
  }

  Future<void> _oauthFinish() async {
    final code = _oauthCodeCtrl.text.trim();
    if (code.isEmpty) return;
    if (_serverCtrl.text.trim().isEmpty) {
      setState(() => _error = 'Enter your server URL first');
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(sessionProvider.notifier).oauthFinish(code, serverUrl: _serverCtrl.text.trim());
    } on AppException catch (e) {
      if (!mounted) return;
      setState(() {
        if (e.code == 'invalid_code') {
          // Surface the state explicitly instead of the opaque backend code: the
          // one-time code is single-use and expires after five minutes.
          _oauthCodeCtrl.clear();
          _error = 'This SSO code has expired or was already used. Start SSO sign-in again to get a fresh code.';
        } else {
          _error = '${e.code}: ${e.message}';
        }
      });
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  /// Signs in with the public demo/review account from `GET /auth/config`.
  /// The demo server URL is the Immich library behind this backend, not the
  /// swipe API base, so the URL field is deliberately left untouched.
  Future<void> _useDemo(AuthConfig config) async {
    if (_busy) return;
    setState(() {
      _createMode = false;
      _userCtrl.text = config.demoUserName;
      _passCtrl.text = config.demoPassword;
      _error = null;
    });
    await _submit();
  }

  @override
  Widget build(BuildContext context) {
    final sessionState = ref.watch(sessionProvider);
    final demoConfig = sessionState.oauthConfig;
    final demoEnabled = _ssoChecked &&
        demoConfig != null &&
        demoConfig.demoEnabled &&
        demoConfig.demoUserName.isNotEmpty &&
        demoConfig.demoPassword.isNotEmpty;
    final oauthEnabled = _ssoChecked && (sessionState.oauthConfig?.oauthEnabled ?? false);
    final oauthText = sessionState.oauthConfig?.oauthButtonText ?? 'SSO';
    final configError = sessionState.configError;
    final scheme = Theme.of(context).colorScheme;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    if (_needsApiKey) {
      return Scaffold(
        appBar: AppBar(title: const Text('Set Immich API Key')),
        body: SafeArea(
          child: SingleChildScrollView(
            padding: EdgeInsets.fromLTRB(16, 16, 16, MediaQuery.of(context).padding.bottom + 16),
            child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: scheme.primaryContainer.withOpacity(isDark ? 0.35 : 1),
                  borderRadius: BorderRadius.circular(16),
                  border: Border.all(color: scheme.outlineVariant.withOpacity(0.3)),
                ),
                child: Row(children: [
                  Icon(Icons.key_rounded, color: scheme.onPrimaryContainer),
                  const SizedBox(width: 12),
                  Expanded(
                    child: Text('Your account needs an Immich API key to continue. Paste it below.',
                        style: Theme.of(context)
                            .textTheme
                            .bodyMedium
                            ?.copyWith(color: scheme.onPrimaryContainer, fontWeight: FontWeight.w600)),
                  ),
                ]),
              ),
              const SizedBox(height: 16),
              TextField(
                  controller: _apiKeyCtrl,
                  decoration: const InputDecoration(labelText: 'Immich API Key', border: OutlineInputBorder())),
              if (_error != null)
                Padding(
                    padding: const EdgeInsets.only(top: 8),
                    child: Text(_error!, style: TextStyle(color: scheme.error))),
              const SizedBox(height: 16),
              SizedBox(
                height: 48,
                child: FilledButton(
                    onPressed: _busy ? null : _submitApiKey,
                    child: _busy
                        ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                        : const Text('Save API Key')),
              ),
            ]),
          ),
        ),
      );
    }

    return Scaffold(
      body: SafeArea(
        child: SingleChildScrollView(
          padding: EdgeInsets.fromLTRB(16, 24, 16, MediaQuery.of(context).padding.bottom + 16),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 520),
            child: Column(crossAxisAlignment: CrossAxisAlignment.stretch, children: [
              // brand header
              Row(children: [
                Container(
                  width: 44,
                  height: 44,
                  decoration: BoxDecoration(
                    borderRadius: BorderRadius.circular(12),
                    boxShadow: [BoxShadow(color: scheme.primary.withOpacity(0.3), blurRadius: 12, offset: const Offset(0, 6))],
                  ),
                  child: ClipRRect(
                    borderRadius: BorderRadius.circular(12),
                    child: Image.asset(
                      'assets/branding/app_icon.png',
                      width: 44,
                      height: 44,
                      fit: BoxFit.cover,
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Text('Immich Swipe',
                      style:
                          Theme.of(context).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w900, letterSpacing: -0.5)),
                  Text('Review your library, one swipe at a time',
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant)),
                ]),
              ]),
              const SizedBox(height: 24),
              // toggle
              Container(
                padding: const EdgeInsets.all(4),
                decoration: BoxDecoration(
                  color: isDark ? Colors.white.withOpacity(0.06) : scheme.surfaceContainerHighest,
                  borderRadius: BorderRadius.circular(14),
                ),
                child: Row(children: [
                  _toggleBtn(label: 'Sign in', selected: !_createMode, onTap: () => setState(() => _createMode = false)),
                  const SizedBox(width: 4),
                  _toggleBtn(label: 'Create account', selected: _createMode, onTap: () => setState(() => _createMode = true)),
                ]),
              ),
              const SizedBox(height: 20),
              if (demoEnabled) ...[
                Container(
                  padding: const EdgeInsets.all(14),
                  decoration: BoxDecoration(
                    color: scheme.primaryContainer.withOpacity(isDark ? 0.3 : 1),
                    borderRadius: BorderRadius.circular(14),
                    border: Border.all(color: scheme.outlineVariant.withOpacity(0.35)),
                  ),
                  child: Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                    Row(children: [
                      Icon(Icons.science_rounded, size: 20, color: scheme.onPrimaryContainer),
                      const SizedBox(width: 8),
                      Expanded(
                        child: Text('Demo account available',
                            style: Theme.of(context).textTheme.titleSmall?.copyWith(
                                  fontWeight: FontWeight.w800,
                                  color: scheme.onPrimaryContainer,
                                )),
                      ),
                    ]),
                    const SizedBox(height: 6),
                    Text(
                        'Try the app with ${demoConfig.demoUserName}. This review-only account is published by the server.',
                        style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onPrimaryContainer)),
                    if (demoConfig.demoServerUrl.isNotEmpty) ...[
                      const SizedBox(height: 4),
                      Text('Demo library: ${demoConfig.demoServerUrl}',
                          style: Theme.of(context)
                              .textTheme
                              .bodySmall
                              ?.copyWith(color: scheme.onPrimaryContainer.withOpacity(0.8), fontSize: 11),
                          overflow: TextOverflow.ellipsis),
                    ],
                    const SizedBox(height: 10),
                    SizedBox(
                      width: double.infinity,
                      height: 44,
                      child: FilledButton.tonal(
                        key: const Key('demo-login-button'),
                        onPressed: _busy ? null : () => _useDemo(demoConfig),
                        child: const Text('Use demo account'),
                      ),
                    ),
                  ]),
                ),
                const SizedBox(height: 16),
              ],
              Text('Use the default instance below, or enter another self-hosted Immich server.',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant, fontSize: 12)),
              const SizedBox(height: 8),
              TextField(
                  controller: _serverCtrl,
                  focusNode: _serverFocus,
                  keyboardType: TextInputType.url,
                  autocorrect: false,
                  decoration: const InputDecoration(
                      labelText: 'Server URL', hintText: 'https://immich.example.com', border: OutlineInputBorder())),
              const SizedBox(height: 12),
              TextField(
                  controller: _userCtrl,
                  decoration: const InputDecoration(labelText: 'User name', border: OutlineInputBorder())),
              const SizedBox(height: 12),
              TextField(
                  controller: _passCtrl,
                  obscureText: _obscure,
                  decoration: InputDecoration(
                    labelText: 'Password',
                    border: const OutlineInputBorder(),
                    suffixIcon: IconButton(
                      icon: Icon(_obscure ? Icons.visibility_off_rounded : Icons.visibility_rounded, size: 20),
                      onPressed: () => setState(() => _obscure = !_obscure),
                    ),
                  )),
              const SizedBox(height: 4),
              if (_createMode) ...[
                const SizedBox(height: 8),
                TextField(
                    controller: _apiKeyCtrl,
                    decoration: const InputDecoration(labelText: 'Immich API Key (optional)', border: OutlineInputBorder())),
                const SizedBox(height: 8),
                Text('Leave empty to add the key later. You’ll be prompted if required (428 api_key_required).',
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant, fontSize: 12)),
              ],
              if (_error != null)
                Container(
                  margin: const EdgeInsets.only(top: 12),
                  padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                  decoration: BoxDecoration(
                    color: scheme.errorContainer.withOpacity(isDark ? 0.35 : 0.9),
                    borderRadius: BorderRadius.circular(12),
                    border: Border.all(color: scheme.error.withOpacity(0.2)),
                  ),
                  child: Row(children: [
                    Icon(Icons.error_outline_rounded, size: 18, color: scheme.onErrorContainer),
                    const SizedBox(width: 8),
                    Expanded(
                        child: Text(_error!,
                            style: Theme.of(context).textTheme.bodySmall?.copyWith(
                                  color: scheme.onErrorContainer,
                                  fontWeight: FontWeight.w600,
                                ))),
                  ]),
                ),
              const SizedBox(height: 16),
              SizedBox(
                height: 48,
                child: FilledButton(
                    onPressed: _busy ? null : _submit,
                    child: _busy
                        ? const SizedBox(width: 16, height: 16, child: CircularProgressIndicator(strokeWidth: 2))
                        : Text(_createMode ? 'Create account' : 'Sign in')),
              ),
              const SizedBox(height: 16),
              Divider(color: scheme.outlineVariant.withOpacity(0.4)),
              if (oauthEnabled) ...[
                const SizedBox(height: 12),
                SizedBox(
                  height: 48,
                  child: OutlinedButton.icon(
                    onPressed: _busy ? null : _oauthStart,
                    icon: const Icon(Icons.shield_rounded, size: 18),
                    label: Text('Sign in with $oauthText'),
                  ),
                ),
                const SizedBox(height: 10),
                TextField(
                    controller: _oauthCodeCtrl,
                    decoration: const InputDecoration(
                        labelText: 'One-time OAuth code (fallback)', border: OutlineInputBorder())),
                const SizedBox(height: 8),
                SizedBox(
                  height: 48,
                  child: OutlinedButton(onPressed: _busy ? null : _oauthFinish, child: const Text('Finish SSO login')),
                ),
                const SizedBox(height: 10),
                Text(
                    'Opens the provider sign-in page in the app and finishes the handoff automatically. '
                    'If your provider blocks the in-app browser, copy the SSO URL from there and paste the code above.',
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant, fontSize: 11)),
              ] else if (_probing)
                Padding(
                  padding: const EdgeInsets.only(top: 12),
                  child: Text('Checking SSO availability…',
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant, fontSize: 12)),
                )
              else if (configError != null)
                Padding(
                  padding: const EdgeInsets.only(top: 12),
                  child: Row(crossAxisAlignment: CrossAxisAlignment.start, children: [
                    Icon(Icons.cloud_off_rounded, size: 16, color: scheme.error),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text('Could not load SSO configuration from this server. Check the URL and try again.',
                          style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.error, fontSize: 12)),
                    ),
                    TextButton(
                      onPressed: _busy ? null : _checkOauthConfig,
                      child: const Text('Retry'),
                    ),
                  ]),
                )
              else if (_ssoChecked)
                Padding(
                  padding: const EdgeInsets.only(top: 12),
                  child: Text('SSO disabled on this server (GET /api/auth/config oauthEnabled=false).',
                      style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant, fontSize: 12)),
                ),
            ]),
          ),
        ),
      ),
    );
  }

  Widget _toggleBtn({required String label, required bool selected, required VoidCallback onTap}) {
    final scheme = Theme.of(context).colorScheme;
    return Expanded(
      child: AnimatedContainer(
        duration: const Duration(milliseconds: 180),
        height: 40,
        decoration: BoxDecoration(
          color: selected ? scheme.primary : Colors.transparent,
          borderRadius: BorderRadius.circular(10),
        ),
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(10),
          child: Center(
            child: Text(label,
                style: Theme.of(context).textTheme.labelLarge?.copyWith(
                      fontWeight: FontWeight.w800,
                      fontSize: 13,
                      color: selected ? scheme.onPrimary : scheme.onSurfaceVariant,
                    )),
          ),
        ),
      ),
    );
  }
}
