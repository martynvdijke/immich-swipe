import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'state/providers.dart';
import 'features/auth/login_screen.dart';
import 'features/review/review_screen.dart';
import 'theme/app_theme.dart';

void main() {
  runApp(const ProviderScope(child: MyApp()));
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
      home: session == null || needsKey ? const LoginScreen() : const ReviewScreen(),
    );
  }
}
