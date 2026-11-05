import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../../core/api_client.dart';
import '../../state/providers.dart';
import 'legal.dart';
import 'legal_webview_screen.dart';

/// Account and settings screen, reachable from the review app bar in every
/// review state (loading, empty, error and deck).
class SettingsScreen extends ConsumerStatefulWidget {
  const SettingsScreen({super.key});

  @override
  ConsumerState<SettingsScreen> createState() => _SettingsScreenState();
}

class _SettingsScreenState extends ConsumerState<SettingsScreen> {
  bool _deleting = false;

  Future<void> _openLegal(String title, String path) async {
    final session = ref.read(sessionProvider).session;
    if (session == null) return;
    await Navigator.of(context).push(
      MaterialPageRoute(
        builder: (_) => LegalWebViewScreen(
          title: title,
          url: legalUrlFor(session.serverUrl, path),
        ),
      ),
    );
  }

  Future<void> _deleteAccount() async {
    final session = ref.read(sessionProvider).session;
    if (session == null || _deleting) return;
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (_) => _DeleteAccountDialog(userName: session.userName),
    );
    if (confirmed != true || !mounted) return;

    setState(() => _deleting = true);
    try {
      await ref
          .read(sessionProvider.notifier)
          .deleteAccount(confirm: session.userName);
      if (!mounted) return;
      Navigator.of(context).popUntil((route) => route.isFirst);
    } on AppException catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Account deletion failed: ${e.code} — ${e.message}'),
        ),
      );
    } catch (e) {
      if (!mounted) return;
      ScaffoldMessenger.of(context)
          .showSnackBar(SnackBar(content: Text('Account deletion failed: $e')));
    } finally {
      if (mounted) setState(() => _deleting = false);
    }
  }

  Future<void> _signOut() async {
    await ref.read(sessionProvider.notifier).logout();
    if (!mounted) return;
    Navigator.of(context).popUntil((route) => route.isFirst);
  }

  @override
  Widget build(BuildContext context) {
    final session = ref.watch(sessionProvider).session;
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      appBar: AppBar(title: const Text('Settings & account')),
      body: SafeArea(
        child: ListView(
          padding: EdgeInsets.fromLTRB(
            16,
            8,
            16,
            MediaQuery.of(context).padding.bottom + 16,
          ),
          children: [
            if (session != null)
              Container(
                padding: const EdgeInsets.all(16),
                decoration: BoxDecoration(
                  color: scheme.surfaceContainerHighest.withValues(alpha: 0.5),
                  borderRadius: BorderRadius.circular(16),
                  border: Border.all(
                    color: scheme.outlineVariant.withValues(alpha: 0.4),
                  ),
                ),
                child: Row(
                  children: [
                    CircleAvatar(
                      radius: 22,
                      backgroundColor: scheme.primaryContainer,
                      child: Icon(
                        Icons.person_rounded,
                        color: scheme.onPrimaryContainer,
                      ),
                    ),
                    const SizedBox(width: 12),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            session.userName,
                            style: Theme.of(context).textTheme.titleMedium
                                ?.copyWith(fontWeight: FontWeight.w800),
                            overflow: TextOverflow.ellipsis,
                          ),
                          const SizedBox(height: 2),
                          Text(
                            session.serverUrl,
                            style: Theme.of(context).textTheme.bodySmall
                                ?.copyWith(color: scheme.onSurfaceVariant),
                            overflow: TextOverflow.ellipsis,
                          ),
                          Text(
                            'Signed in with ${session.mode}',
                            style: Theme.of(context).textTheme.bodySmall
                                ?.copyWith(
                                  color: scheme.onSurfaceVariant,
                                  fontSize: 11,
                                ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            const SizedBox(height: 16),
            _sectionLabel(context, 'Legal'),
            _tile(
              context,
              icon: Icons.privacy_tip_outlined,
              title: 'Privacy',
              subtitle: 'How this app handles your data',
              onTap: () => _openLegal('Privacy', '/privacy'),
            ),
            _tile(
              context,
              icon: Icons.description_outlined,
              title: 'Terms',
              subtitle: 'Terms of service',
              onTap: () => _openLegal('Terms', '/terms'),
            ),
            const SizedBox(height: 16),
            _sectionLabel(context, 'Account'),
            _tile(
              context,
              icon: Icons.delete_forever_outlined,
              title: 'Delete account',
              subtitle:
                  'Remove your account, sessions and on-device review history',
              destructive: true,
              enabled: !_deleting,
              onTap: _deleteAccount,
            ),
            _tile(
              context,
              icon: Icons.logout_rounded,
              title: 'Sign out',
              subtitle: 'Keep your account on this server',
              onTap: _signOut,
            ),
            if (_deleting)
              const Padding(
                padding: EdgeInsets.only(top: 16),
                child: Center(child: CircularProgressIndicator()),
              ),
            const SizedBox(height: 20),
            Text(
              'Photos and albums stay on your Immich server. Deleting your Swipe account removes your account, sessions, counters and on-device review history, but does not delete your Immich account or photos.',
              style: Theme.of(context).textTheme.bodySmall
                  ?.copyWith(color: scheme.onSurfaceVariant, fontSize: 11),
            ),
          ],
        ),
      ),
    );
  }

  Widget _sectionLabel(BuildContext context, String text) => Padding(
    padding: const EdgeInsets.only(left: 4, bottom: 6),
    child: Text(
      text,
      style: Theme.of(context).textTheme.labelSmall?.copyWith(
        fontWeight: FontWeight.w800,
        letterSpacing: 0.6,
        color: Theme.of(context).colorScheme.primary,
      ),
    ),
  );

  Widget _tile(
    BuildContext context, {
    required IconData icon,
    required String title,
    String? subtitle,
    required VoidCallback onTap,
    bool destructive = false,
    bool enabled = true,
  }) {
    final scheme = Theme.of(context).colorScheme;
    final color = destructive ? scheme.error : scheme.onSurface;
    return Card(
      elevation: 0,
      margin: const EdgeInsets.only(bottom: 8),
      color: scheme.surfaceContainerHighest.withValues(alpha: 0.35),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(14),
        side: BorderSide(color: scheme.outlineVariant.withValues(alpha: 0.4)),
      ),
      child: ListTile(
        enabled: enabled,
        leading: Icon(icon, color: color),
        title: Text(
          title,
          style: TextStyle(color: color, fontWeight: FontWeight.w600),
        ),
        subtitle: subtitle == null ? null : Text(subtitle),
        trailing: const Icon(Icons.chevron_right_rounded),
        onTap: enabled ? onTap : null,
      ),
    );
  }
}

class _DeleteAccountDialog extends StatefulWidget {
  const _DeleteAccountDialog({required this.userName});
  final String userName;

  @override
  State<_DeleteAccountDialog> createState() => _DeleteAccountDialogState();
}

class _DeleteAccountDialogState extends State<_DeleteAccountDialog> {
  final _controller = TextEditingController();

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  bool get _matches => _controller.text.trim() == widget.userName;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return AlertDialog(
      title: const Text('Delete account?'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text(
            'This removes your Immich Swipe account, saved sessions and counters from this server, '
            'and clears this device’s review history. Your Immich account and photos are not deleted.',
          ),
          const SizedBox(height: 12),
          Text(
            'Type ${widget.userName} to confirm.',
            style: Theme.of(context).textTheme.bodySmall
                ?.copyWith(color: scheme.onSurfaceVariant),
          ),
          const SizedBox(height: 8),
          TextField(
            key: const Key('delete-confirm-field'),
            controller: _controller,
            autofocus: true,
            autocorrect: false,
            decoration: const InputDecoration(
              labelText: 'User name',
              border: OutlineInputBorder(),
            ),
            onChanged: (_) => setState(() {}),
          ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(false),
          child: const Text('Cancel'),
        ),
        FilledButton(
          key: const Key('delete-confirm-button'),
          style: FilledButton.styleFrom(
            backgroundColor: scheme.error,
            foregroundColor: scheme.onError,
          ),
          onPressed: _matches ? () => Navigator.of(context).pop(true) : null,
          child: const Text('Delete account'),
        ),
      ],
    );
  }
}
