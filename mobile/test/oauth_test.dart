import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:immich_swipe/core/format.dart';
import 'package:immich_swipe/core/oauth.dart';
import 'package:immich_swipe/core/secure_store.dart';
import 'package:immich_swipe/features/auth/login_screen.dart';
import 'package:immich_swipe/models/session.dart';
import 'package:immich_swipe/state/providers.dart';

class _FakeStore extends SecureStore {
  @override
  Future<Map<String, String?>> readSession() async => {};
  @override
  Future<void> saveSession(
      {required String serverUrl, required String token, required String userName, required String mode}) async {}
  @override
  Future<void> clearSession() async {}
  @override
  Future<Set<String>> readReviewed(String a, String b) async => {};
  @override
  Future<void> writeReviewed(String a, String b, Set<String> ids) async {}
  @override
  Future<void> addReviewed(String a, String b, String id) async {}
}

/// Resolves every request immediately (config included) so `SessionNotifier`
/// never leaves a pending HTTP timer behind in the fake-async test zone.
Dio _stubDio({required bool oauthEnabled}) {
  final d = Dio(BaseOptions(baseUrl: 'http://127.0.0.1:1/api'));
  d.interceptors.add(InterceptorsWrapper(onRequest: (o, h) {
    if (o.path.contains('/auth/config')) {
      h.resolve(Response(
          requestOptions: o,
          statusCode: 200,
          data: {'oauthEnabled': oauthEnabled, 'oauthButtonText': 'MockSSO'}));
    } else {
      h.resolve(Response(requestOptions: o, statusCode: 200, data: {}));
    }
  }));
  return d;
}

void main() {
  group('oauthCodeFromUrl', () {
    test('extracts the one-time handoff code from the backend redirect', () {
      expect(oauthCodeFromUrl('http://10.0.2.2:8080/login?oauthCode=abc123'), 'abc123');
      expect(oauthCodeFromUrl('https://swipe.example.com/login?oauthCode=x-y_z.1'), 'x-y_z.1');
    });

    test('returns null for anything that is not the redirect target', () {
      expect(oauthCodeFromUrl('http://10.0.2.2:8080/mock-idp?state=s1'), isNull);
      expect(oauthCodeFromUrl('https://immich.example.com/api/oauth/authorize?code_challenge=xyz'), isNull);
      expect(oauthCodeFromUrl('http://10.0.2.2:8080/login'), isNull);
      expect(oauthCodeFromUrl('http://10.0.2.2:8080/login?oauthCode='), isNull);
      expect(oauthCodeFromUrl('not a url at all'), isNull);
    });
  });

  group('oauthErrorFromUrl', () {
    test('reads the oauthError and error parameters', () {
      expect(oauthErrorFromUrl('http://h/login?oauthError=access_denied'), 'access_denied');
      expect(oauthErrorFromUrl('http://h/login?error=invalid_scope'), 'invalid_scope');
      expect(oauthErrorFromUrl('http://h/login?oauthCode=abc'), isNull);
      expect(oauthErrorFromUrl('http://h/login?oauthError='), isNull);
    });
  });

  group('formatDuration', () {
    test('renders the Immich H:MM:SS.ffffff duration like a player', () {
      expect(formatDuration('0:00:12.000000'), '0:12');
      expect(formatDuration('0:01:05.500000'), '1:05');
      expect(formatDuration('1:02:03.000000'), '62:03');
    });

    test('falls back safely for missing or unparseable values', () {
      expect(formatDuration(null), 'Video');
      expect(formatDuration(''), 'Video');
      expect(formatDuration('0:00:00.000000'), 'Video');
      expect(formatDuration('12'), '12');
    });
  });

  testWidgets('LoginScreen surfaces the SSO button when the server enables OAuth', (t) async {
    final oldOnError = FlutterError.onError;
    FlutterError.onError = (d) {
      if (!d.toString().contains('overflowed')) oldOnError?.call(d);
    };
    addTearDown(() => FlutterError.onError = oldOnError);
    final container = ProviderContainer(overrides: [
      dioProvider.overrideWithValue(_stubDio(oauthEnabled: true)),
      secureStoreProvider.overrideWithValue(_FakeStore()),
    ]);
    addTearDown(container.dispose);
    container.read(sessionProvider.notifier).state =
        SessionState(oauthConfig: AuthConfig(oauthEnabled: true, oauthButtonText: 'MockSSO'));
    await t.pumpWidget(
        UncontrolledProviderScope(container: container, child: const MaterialApp(home: LoginScreen())));
    await t.pump();
    await t.pump(const Duration(milliseconds: 50));

    expect(find.text('Sign in with MockSSO'), findsOneWidget);
    expect(find.text('Finish SSO login'), findsOneWidget);

    // With no server URL there is nothing to start SSO against: the guard message
    // must appear instead of pushing a WebView.
    await t.ensureVisible(find.text('Sign in with MockSSO'));
    await t.tap(find.text('Sign in with MockSSO'));
    await t.pump();
    expect(find.textContaining('Enter your server URL first'), findsOneWidget);
  });

  testWidgets('LoginScreen explains when the server has SSO disabled', (t) async {
    final oldOnError = FlutterError.onError;
    FlutterError.onError = (d) {
      if (!d.toString().contains('overflowed')) oldOnError?.call(d);
    };
    addTearDown(() => FlutterError.onError = oldOnError);
    final container = ProviderContainer(overrides: [
      dioProvider.overrideWithValue(_stubDio(oauthEnabled: false)),
      secureStoreProvider.overrideWithValue(_FakeStore()),
    ]);
    addTearDown(container.dispose);
    container.read(sessionProvider.notifier).state =
        SessionState(oauthConfig: AuthConfig(oauthEnabled: false, oauthButtonText: 'SSO'));
    await t.pumpWidget(
        UncontrolledProviderScope(container: container, child: const MaterialApp(home: LoginScreen())));
    await t.pump();
    await t.pump(const Duration(milliseconds: 50));

    expect(find.textContaining('SSO disabled on this server'), findsOneWidget);
    expect(find.text('Finish SSO login'), findsNothing);
  });
}
