import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'core/telemetry.dart';
import 'state/providers.dart';
import 'features/auth/login_screen.dart';
import 'features/review/review_screen.dart';
import 'theme/app_theme.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  // No-op unless OTEL_EXPORTER_OTLP_ENDPOINT is provided at build time.
  await Telemetry.init(serviceVersion: '1.17.0');
  WidgetsBinding.instance.addObserver(_TelemetryLifecycleObserver());
  runApp(
    ProviderScope(
      // Preserve v2 behaviour: surface errors instead of silently retrying forever.
      retry: (retryCount, error) => null,
      child: const MyApp(),
    ),
  );
}

/// Batch exporters are timer-driven and those timers do not fire while the app
/// is backgrounded, so flush pending spans/metrics/logs when the app pauses.
class _TelemetryLifecycleObserver with WidgetsBindingObserver {
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.paused ||
        state == AppLifecycleState.detached) {
      Telemetry.flush();
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
