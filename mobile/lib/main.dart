import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/app_config.dart';
import 'core/secure_store.dart';
import 'src/telemetry/telemetry.dart';
import 'state/providers.dart';
import 'features/auth/login_screen.dart';
import 'features/review/review_screen.dart';
import 'theme/app_theme.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  WidgetsBinding.instance.addObserver(_TelemetryLifecycleObserver());
  runApp(
    ProviderScope(
      // Preserve v2 behaviour: surface errors instead of silently retrying forever.
      retry: (retryCount, error) => null,
      child: const _TelemetryBootstrap(child: MyApp()),
    ),
  );
}

/// Starts telemetry after the first frame has been scheduled.
///
/// The saved backend URL decides where the collector endpoint is fetched from.
/// Both the storage read and the network call are fire-and-forget: nothing here
/// can block the first frame or fail startup while offline.
class _TelemetryBootstrap extends StatefulWidget {
  const _TelemetryBootstrap({required this.child});

  final Widget child;

  @override
  State<_TelemetryBootstrap> createState() => _TelemetryBootstrapState();
}

class _TelemetryBootstrapState extends State<_TelemetryBootstrap> {
  @override
  void initState() {
    super.initState();
    unawaited(_startTelemetry());
  }

  Future<void> _startTelemetry() async {
    var baseUrl = kDefaultServerUrl;
    try {
      final saved = await SecureStore().readLastServerUrl();
      if (saved != null && saved.trim().isNotEmpty) baseUrl = saved;
    } catch (_) {
      // Secure storage unavailable (e.g. first launch after restore).
    }
    try {
      await Telemetry.instance.startFromServer(baseUrl);
    } catch (_) {
      // Telemetry must never affect startup.
    }
  }

  @override
  Widget build(BuildContext context) => widget.child;
}

/// Batch exporters are timer-driven and those timers do not fire while the app
/// is backgrounded, so flush pending spans when the app pauses.
class _TelemetryLifecycleObserver with WidgetsBindingObserver {
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.paused ||
        state == AppLifecycleState.detached) {
      Telemetry.instance.flush();
    }
  }
}

class MyApp extends ConsumerWidget {
  const MyApp({super.key});
  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final sessState = ref.watch(sessionProvider);
    final session = sessState.session;
    final needsKey = sessState.needsApiKey;
    return MaterialApp(
      title: 'Immich Swipe',
      theme: AppTheme.light,
      darkTheme: AppTheme.dark,
      themeMode: ThemeMode.dark,
      home: session == null || needsKey
          ? const LoginScreen()
          : const ReviewScreen(),
    );
  }
}
