import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:immich_swipe/core/secure_store.dart';
import 'package:immich_swipe/features/auth/login_screen.dart';
import 'package:immich_swipe/state/providers.dart';

class _FakeStore extends SecureStore {
  String? savedUrl;

  @override
  Future<Map<String, String?>> readSession() async => {};

  @override
  Future<void> saveSession(
      {required String serverUrl, required String token, required String userName, required String mode}) async {
    savedUrl = serverUrl;
  }

  @override
  Future<void> clearSession() async {}

  @override
  Future<Set<String>> readReviewed(String a, String b) async => {};

  @override
  Future<void> writeReviewed(String a, String b, Set<String> ids) async {}

  @override
  Future<void> addReviewed(String a, String b, String id) async {}
}

/// Records every request and answers immediately, so no HTTP timer is pending.
Dio _capturingDio(List<RequestOptions> requests,
    {Map<String, dynamic> Function(RequestOptions)? respond}) {
  final d = Dio(BaseOptions(baseUrl: 'http://example.com/api'));
  d.interceptors.add(InterceptorsWrapper(onRequest: (o, h) {
    requests.add(o);
    final data = respond != null ? respond(o) : <String, dynamic>{};
    h.resolve(Response(requestOptions: o, statusCode: 200, data: data));
  }));
  return d;
}

void main() {
  test('SessionNotifier.login omits serverUrl and stores the typed backend URL', () async {
    final store = _FakeStore();
    final requests = <RequestOptions>[];
    final dio = _capturingDio(requests, respond: (o) {
      if (o.path.contains('/auth/login')) {
        return {
          'token': 'tok',
          'userName': 'alice',
          // The backend resolves the Immich server; this must not become the API base.
          'serverUrl': 'https://immich.example.com',
          'mode': 'apiKey',
          'hasApiKey': true,
        };
      }
      return {};
    });
    final c = ProviderContainer(overrides: [
      authDioFactoryProvider.overrideWithValue((_, _) => dio),
      secureStoreProvider.overrideWithValue(store),
    ]);
    addTearDown(c.dispose);

    await c.read(sessionProvider.notifier).login(
          serverUrl: 'https://swipe.example.com',
          userName: 'alice',
          password: 'secret123',
        );

    final login = requests.singleWhere((r) => r.path.contains('/auth/login'));
    final body = login.data as Map;
    expect(body.containsKey('serverUrl'), isFalse);
    expect(body['userName'], 'alice');
    expect(body['password'], 'secret123');

    expect(store.savedUrl, 'https://swipe.example.com');
    expect(c.read(sessionProvider).session?.serverUrl, 'https://swipe.example.com');
  });

  test('oauthStart posts an empty body without serverUrl', () async {
    final store = _FakeStore();
    final requests = <RequestOptions>[];
    final dio = _capturingDio(requests, respond: (o) {
      if (o.path.contains('/auth/oauth/start')) {
        return {'url': 'https://idp.example.com/authorize'};
      }
      return {};
    });
    final c = ProviderContainer(overrides: [
      authDioFactoryProvider.overrideWithValue((_, _) => dio),
      secureStoreProvider.overrideWithValue(store),
    ]);
    addTearDown(c.dispose);

    final res = await c.read(sessionProvider.notifier).oauthStart(serverUrl: 'https://swipe.example.com');
    expect(res['url'], 'https://idp.example.com/authorize');

    final start = requests.singleWhere((r) => r.path.contains('/auth/oauth/start'));
    final body = start.data;
    expect(body, isA<Map>());
    expect((body as Map).containsKey('serverUrl'), isFalse);
    expect(body, isEmpty);
  });

  test('fetchConfigFor("") performs no request and returns false', () async {
    final store = _FakeStore();
    final requests = <RequestOptions>[];
    final dio = _capturingDio(requests);
    final c = ProviderContainer(overrides: [
      authDioFactoryProvider.overrideWithValue((_, _) => dio),
      secureStoreProvider.overrideWithValue(store),
    ]);
    addTearDown(c.dispose);

    final ok = await c.read(sessionProvider.notifier).fetchConfigFor('');
    expect(ok, isFalse);
    expect(requests, isEmpty);
    expect(c.read(sessionProvider).oauthConfig, isNull);

    final okNull = await c.read(sessionProvider.notifier).fetchConfigFor(null);
    expect(okNull, isFalse);
    expect(requests, isEmpty);
  });

  testWidgets('LoginScreen reveals SSO after typing a server URL (debounced probe)', (t) async {
    final oldOnError = FlutterError.onError;
    FlutterError.onError = (d) {
      if (!d.toString().contains('overflowed')) oldOnError?.call(d);
    };
    addTearDown(() => FlutterError.onError = oldOnError);

    final requests = <RequestOptions>[];
    final dio = _capturingDio(requests, respond: (o) {
      if (o.path.contains('/auth/config')) {
        return {'oauthEnabled': true, 'oauthButtonText': 'MockSSO'};
      }
      return {};
    });
    final container = ProviderContainer(overrides: [
      authDioFactoryProvider.overrideWithValue((_, _) => dio),
      secureStoreProvider.overrideWithValue(_FakeStore()),
    ]);
    addTearDown(container.dispose);

    await t.pumpWidget(
        UncontrolledProviderScope(container: container, child: const MaterialApp(home: LoginScreen())));
    await t.pump();

    // Nothing is claimed before the user provides a server URL.
    expect(find.text('Checking SSO availability…'), findsNothing);
    expect(find.text('Sign in with MockSSO'), findsNothing);

    await t.enterText(find.widgetWithText(TextField, 'Server URL'), 'https://swipe.example.com');
    await t.pump();
    expect(find.text('Checking SSO availability…'), findsOneWidget);

    await t.pump(const Duration(milliseconds: 500));
    await t.pump();

    expect(requests.any((r) => r.path.contains('/auth/config')), isTrue);
    expect(find.text('Sign in with MockSSO'), findsOneWidget);
    expect(find.text('Finish SSO login'), findsOneWidget);
  });
}
