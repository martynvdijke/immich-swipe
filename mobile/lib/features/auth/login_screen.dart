// ignore_for_file: deprecated_member_use, use_build_context_synchronously
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../state/providers.dart';
import '../../core/api_client.dart';

class LoginScreen extends ConsumerStatefulWidget {
  const LoginScreen({super.key});
  @override
  ConsumerState<LoginScreen> createState() => _LoginScreenState();
}

class _LoginScreenState extends ConsumerState<LoginScreen> {
  final _serverCtrl = TextEditingController(text: 'http://10.0.2.2:8080');
  final _userCtrl = TextEditingController();
  final _passCtrl = TextEditingController();
  final _apiKeyCtrl = TextEditingController();
  final _oauthCodeCtrl = TextEditingController();
  bool _createMode = false;
  bool _busy = false;
  String? _error;
  bool _needsApiKey = false;
  bool _obscure = true;

  @override
  void dispose() {
    _serverCtrl.dispose();
    _userCtrl.dispose();
    _passCtrl.dispose();
    _apiKeyCtrl.dispose();
    _oauthCodeCtrl.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final url = _serverCtrl.text.trim();
      final user = _userCtrl.text.trim();
      final pass = _passCtrl.text;
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

  Future<void> _oauthStart() async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final res = await ref.read(sessionProvider.notifier).oauthStart();
      final url = res['url'] as String? ?? '';
      if (!mounted) return;
      await showDialog(
          context: context,
          builder: (c) => AlertDialog(
                title: const Text('SSO Login'),
                content: SelectableText('Open this URL in a browser:\n\n$url\n\nThen paste the one-time code below.'),
                actions: [
                  TextButton(
                      onPressed: () {
                        Clipboard.setData(ClipboardData(text: url));
                        Navigator.pop(c);
                      },
                      child: const Text('Copy URL')),
                  TextButton(onPressed: () => Navigator.pop(c), child: const Text('Close')),
                ],
              ));
    } on AppException catch (e) {
      setState(() => _error = '${e.code}: ${e.message}');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _oauthFinish() async {
    final code = _oauthCodeCtrl.text.trim();
    if (code.isEmpty) return;
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await ref.read(sessionProvider.notifier).oauthFinish(code, serverUrl: _serverCtrl.text.trim());
    } on AppException catch (e) {
      setState(() => _error = '${e.code}: ${e.message}');
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final oauthEnabled = ref.watch(sessionProvider).oauthConfig?.oauthEnabled ?? false;
    final oauthText = ref.watch(sessionProvider).oauthConfig?.oauthButtonText ?? 'SSO';
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
                    gradient: const LinearGradient(colors: [Color(0xFF6366F1), Color(0xFF8B5CF6)]),
                    borderRadius: BorderRadius.circular(12),
                    boxShadow: [BoxShadow(color: scheme.primary.withOpacity(0.3), blurRadius: 12, offset: const Offset(0, 6))],
                  ),
                  child: const Icon(Icons.style_rounded, color: Colors.white, size: 22),
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
              TextField(
                  controller: _serverCtrl,
                  decoration: const InputDecoration(
                      labelText: 'Server URL', hintText: 'http://10.0.2.2:8080', border: OutlineInputBorder())),
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
                    decoration: const InputDecoration(labelText: 'Paste one-time OAuth code', border: OutlineInputBorder())),
                const SizedBox(height: 8),
                SizedBox(
                  height: 48,
                  child: OutlinedButton(onPressed: _busy ? null : _oauthFinish, child: const Text('Finish SSO login')),
                ),
                const SizedBox(height: 10),
                Text(
                    'This build shows the SSO URL to open externally. Complete the flow in a browser and paste the code above.',
                    style: Theme.of(context).textTheme.bodySmall?.copyWith(color: scheme.onSurfaceVariant, fontSize: 11)),
              ] else
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
