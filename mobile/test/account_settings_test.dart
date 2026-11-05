import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:immich_swipe/core/secure_store.dart';
import 'package:immich_swipe/data/album_repository.dart';
import 'package:immich_swipe/data/asset_repository.dart';
import 'package:immich_swipe/features/auth/login_screen.dart';
import 'package:immich_swipe/features/review/review_screen.dart';
import 'package:immich_swipe/features/settings/legal.dart';
import 'package:immich_swipe/features/settings/settings_screen.dart';
import 'package:immich_swipe/models/immich_album.dart';
import 'package:immich_swipe/models/immich_asset.dart';
import 'package:immich_swipe/models/session.dart';
import 'package:immich_swipe/state/providers.dart';

import 'dart:typed_data';

class _FakeStore extends SecureStore {
  bool cleared = false;
  bool reviewedCleared = false;

  @override
  Future<Map<String, String?>> readSession() async => const {};
  @override
  Future<String?> readLastServerUrl() async => null;
  @override
  Future<void> saveSession({
    required String serverUrl,
    required String token,
    required String userName,
    required String mode,
  }) async {}
  @override
  Future<void> clearSession() async => cleared = true;
  @override
  Future<void> clearReviewed(String serverUrl, String userName) async =>
      reviewedCleared = true;
  @override
  Future<Set<String>> readReviewed(String a, String b) async => {};
  @override
  Future<void> writeReviewed(String a, String b, Set<String> ids) async {}
  @override
  Future<void> addReviewed(String a, String b, String id) async {}
}

class _EmptyAssets extends AssetRepository {
  _EmptyAssets() : super(Dio());
  @override
  Future<List<ImmichAsset>> searchRandom({
    required int size,
    bool imageOnly = false,
  }) async => [];
  @override
  Future<Uint8List> fetchThumbnail(
    String id, {
    String size = 'preview',
  }) async => Uint8List(0);
}

class _EmptyAlbums extends AlbumRepository {
  _EmptyAlbums() : super(Dio());
  @override
  Future<List<ImmichAlbum>> listAlbums() async => [];
  @override
  Future<void> addToAlbum(String albumId, String assetId) async {}
}

/// Records requests and answers immediately with [respond].
Dio _capturingDio(
  List<RequestOptions> requests, {
  Map<String, dynamic> Function(RequestOptions)? respond,
  int status = 200,
}) {
  final d = Dio(BaseOptions(baseUrl: 'http://example.com/api'));
  d.interceptors.add(
    InterceptorsWrapper(
      onRequest: (o, h) {
        requests.add(o);
        h.resolve(
          Response(
            requestOptions: o,
            statusCode: status,
            data: respond == null ? <String, dynamic>{} : respond(o),
          ),
        );
      },
    ),
  );
  return d;
}

SessionInfo _session({String userName = 'alice'}) => SessionInfo(
  serverUrl: 'https://swipe.example.com',
  token: 'tok',
  userName: userName,
  mode: 'apiKey',
);

void main() {
  group('legalUrlFor', () {
    test(
      'joins the backend base and a legal path without duplicate slashes',
      () {
        expect(
          legalUrlFor('https://swipe.example.com', '/privacy'),
          'https://swipe.example.com/privacy',
        );
        expect(
          legalUrlFor('https://swipe.example.com/', '/terms'),
          'https://swipe.example.com/terms',
        );
        expect(
          legalUrlFor('https://swipe.example.com/base', 'privacy'),
          'https://swipe.example.com/base/privacy',
        );
      },
    );
  });

  group('AuthConfig demo fields', () {
    test('parses the public demo credentials from /auth/config', () {
      final cfg = AuthConfig.fromJson({
        'oauthEnabled': false,
        'oauthButtonText': 'SSO',
        'demoEnabled': true,
        'demoUserName': 'reviewer',
        'demoPassword': 'review-pass',
        'demoServerUrl': 'https://immich.example.com',
      });
      expect(cfg.demoEnabled, isTrue);
      expect(cfg.demoUserName, 'reviewer');
      expect(cfg.demoPassword, 'review-pass');
      expect(cfg.demoServerUrl, 'https://immich.example.com');
    });

    test('defaults to no demo account when the fields are absent', () {
      final cfg = AuthConfig.fromJson({'oauthEnabled': false});
      expect(cfg.demoEnabled, isFalse);
      expect(cfg.demoUserName, isEmpty);
      expect(cfg.demoPassword, isEmpty);
      expect(cfg.demoServerUrl, isEmpty);
    });
  });

  testWidgets('demo login fills the public credentials and signs in', (
    t,
  ) async {
    final oldOnError = FlutterError.onError;
    FlutterError.onError = (d) {
      if (!d.toString().contains('overflowed')) oldOnError?.call(d);
    };
    addTearDown(() => FlutterError.onError = oldOnError);

    final requests = <RequestOptions>[];
    final dio = _capturingDio(
      requests,
      respond: (o) {
        if (o.path.contains('/auth/config')) {
          return {
            'oauthEnabled': false,
            'demoEnabled': true,
            'demoUserName': 'reviewer',
            'demoPassword': 'review-pass',
            'demoServerUrl': 'https://immich.example.com',
          };
        }
        if (o.path.contains('/auth/login')) {
          return {
            'token': 'tok',
            'userName': 'reviewer',
            'serverUrl': 'https://immich.example.com',
            'mode': 'apiKey',
            'hasApiKey': true,
          };
        }
        return {};
      },
    );
    final container = ProviderContainer(
      overrides: [
        authDioFactoryProvider.overrideWithValue((_, _) => dio),
        secureStoreProvider.overrideWithValue(_FakeStore()),
      ],
    );
    addTearDown(container.dispose);

    await t.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(home: LoginScreen()),
      ),
    );
    await t.pump();
    await t.pump(const Duration(milliseconds: 500));
    await t.pump();

    expect(find.text('Demo account available'), findsOneWidget);
    await t.ensureVisible(find.byKey(const Key('demo-login-button')));
    await t.tap(find.byKey(const Key('demo-login-button')));
    await t.pump();
    await t.pump(const Duration(milliseconds: 50));

    final login = requests.singleWhere((r) => r.path.contains('/auth/login'));
    final body = login.data as Map;
    expect(body['userName'], 'reviewer');
    expect(body['password'], 'review-pass');
    expect(body.containsKey('serverUrl'), isFalse);
    expect(container.read(sessionProvider).session?.userName, 'reviewer');
  });

  testWidgets('a config probe failure is shown distinctly and can be retried', (
    t,
  ) async {
    final oldOnError = FlutterError.onError;
    FlutterError.onError = (d) {
      if (!d.toString().contains('overflowed')) oldOnError?.call(d);
    };
    addTearDown(() => FlutterError.onError = oldOnError);

    var configCalls = 0;
    final dio = Dio(BaseOptions(baseUrl: 'http://example.com/api'));
    dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (o, h) {
          configCalls++;
          h.reject(
            DioException(
              requestOptions: o,
              response: Response(
                requestOptions: o,
                statusCode: 500,
                data: {'message': 'backend down'},
              ),
              type: DioExceptionType.badResponse,
            ),
          );
        },
      ),
    );
    final container = ProviderContainer(
      overrides: [
        authDioFactoryProvider.overrideWithValue((_, _) => dio),
        secureStoreProvider.overrideWithValue(_FakeStore()),
      ],
    );
    addTearDown(container.dispose);

    await t.pumpWidget(
      UncontrolledProviderScope(
        container: container,
        child: const MaterialApp(home: LoginScreen()),
      ),
    );
    await t.pump();
    await t.pump(const Duration(milliseconds: 500));
    await t.pump();

    expect(
      find.textContaining('Could not load SSO configuration'),
      findsOneWidget,
    );
    expect(find.textContaining('SSO disabled on this server'), findsNothing);
    final before = configCalls;
    await t.tap(find.widgetWithText(TextButton, 'Retry'));
    await t.pump();
    await t.pump(const Duration(milliseconds: 500));
    await t.pump();
    expect(configCalls, greaterThan(before));
    expect(
      find.textContaining('Could not load SSO configuration'),
      findsOneWidget,
    );
  });

  testWidgets(
    'an invalid/expired SSO code gets an explicit, actionable state',
    (t) async {
      final oldOnError = FlutterError.onError;
      FlutterError.onError = (d) {
        if (!d.toString().contains('overflowed')) oldOnError?.call(d);
      };
      addTearDown(() => FlutterError.onError = oldOnError);

      final dio = Dio(BaseOptions(baseUrl: 'http://example.com/api'));
      dio.interceptors.add(
        InterceptorsWrapper(
          onRequest: (o, h) {
            if (o.path.contains('/auth/config')) {
              h.resolve(
                Response(
                  requestOptions: o,
                  statusCode: 200,
                  data: {'oauthEnabled': true, 'oauthButtonText': 'MockSSO'},
                ),
              );
            } else if (o.path.contains('/auth/oauth/finish')) {
              h.reject(
                DioException(
                  requestOptions: o,
                  response: Response(
                    requestOptions: o,
                    statusCode: 400,
                    data: {
                      'code': 'invalid_code',
                      'error': 'invalid or expired code',
                    },
                  ),
                  type: DioExceptionType.badResponse,
                ),
              );
            } else {
              h.resolve(Response(requestOptions: o, statusCode: 200, data: {}));
            }
          },
        ),
      );
      final container = ProviderContainer(
        overrides: [
          authDioFactoryProvider.overrideWithValue((_, _) => dio),
          secureStoreProvider.overrideWithValue(_FakeStore()),
        ],
      );
      addTearDown(container.dispose);

      await t.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(home: LoginScreen()),
        ),
      );
      await t.pump();
      await t.pump(const Duration(milliseconds: 500));
      await t.pump();

      await t.enterText(
        find.widgetWithText(TextField, 'One-time OAuth code (fallback)'),
        'used-code',
      );
      await t.ensureVisible(find.text('Finish SSO login'));
      await t.tap(find.text('Finish SSO login'));
      await t.pump();
      await t.pump(const Duration(milliseconds: 100));

      expect(
        find.textContaining('expired or was already used'),
        findsOneWidget,
      );
      final field = t.widget<TextField>(
        find.widgetWithText(TextField, 'One-time OAuth code (fallback)'),
      );
      expect(field.controller?.text, isEmpty);
    },
  );

  group('Settings / Account', () {
    ProviderContainer make({
      required List<RequestOptions> requests,
      _FakeStore? store,
    }) {
      final container = ProviderContainer(
        overrides: [
          authDioFactoryProvider.overrideWithValue(
            (_, _) => _capturingDio(requests),
          ),
          secureStoreProvider.overrideWithValue(store ?? _FakeStore()),
        ],
      );
      container.read(sessionProvider.notifier).state = SessionState(
        session: _session(),
      );
      return container;
    }

    testWidgets('shows Privacy, Terms, Delete account and Sign out', (t) async {
      final container = make(requests: []);
      addTearDown(container.dispose);
      await t.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(home: SettingsScreen()),
        ),
      );
      await t.pump();

      expect(find.text('Privacy'), findsOneWidget);
      expect(find.text('Terms'), findsOneWidget);
      expect(find.text('Delete account'), findsOneWidget);
      expect(find.text('Sign out'), findsOneWidget);
      expect(find.text('alice'), findsOneWidget);
    });

    testWidgets(
      'delete account requires typing the exact user name, then clears the local session',
      (t) async {
        final requests = <RequestOptions>[];
        final store = _FakeStore();
        final container = make(requests: requests, store: store);
        addTearDown(container.dispose);

        await t.pumpWidget(
          UncontrolledProviderScope(
            container: container,
            child: const MaterialApp(home: SettingsScreen()),
          ),
        );
        await t.pump();

        await t.tap(find.text('Delete account'));
        await t.pump();
        await t.pump(const Duration(milliseconds: 200));

        FilledButton confirmButton() => t.widget<FilledButton>(
          find.byKey(const Key('delete-confirm-button')),
        );
        expect(confirmButton().onPressed, isNull);

        await t.enterText(
          find.byKey(const Key('delete-confirm-field')),
          'wrong',
        );
        await t.pump();
        expect(confirmButton().onPressed, isNull);

        await t.enterText(
          find.byKey(const Key('delete-confirm-field')),
          'alice',
        );
        await t.pump();
        expect(confirmButton().onPressed, isNotNull);

        await t.tap(find.byKey(const Key('delete-confirm-button')));
        await t.pump();
        await t.pump(const Duration(milliseconds: 100));

        final delete = requests.singleWhere(
          (r) => r.path.contains('/auth/account') && r.method == 'DELETE',
        );
        expect((delete.data as Map)['confirm'], 'alice');
        expect(store.cleared, isTrue);
        expect(store.reviewedCleared, isTrue);
        expect(container.read(sessionProvider).session, isNull);
      },
    );

    testWidgets('sign out calls the backend and clears the local session', (
      t,
    ) async {
      final requests = <RequestOptions>[];
      final store = _FakeStore();
      final container = make(requests: requests, store: store);
      addTearDown(container.dispose);

      await t.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(home: SettingsScreen()),
        ),
      );
      await t.pump();

      await t.tap(find.text('Sign out'));
      await t.pump();
      await t.pump(const Duration(milliseconds: 100));

      expect(requests.any((r) => r.path.contains('/auth/logout')), isTrue);
      expect(store.cleared, isTrue);
      expect(container.read(sessionProvider).session, isNull);
    });
  });

  testWidgets(
    'the settings/account menu is available on the empty review screen',
    (t) async {
      final requests = <RequestOptions>[];
      final dio = _capturingDio(requests);
      final container = ProviderContainer(
        overrides: [
          dioProvider.overrideWithValue(dio),
          secureStoreProvider.overrideWithValue(_FakeStore()),
          assetRepositoryProvider.overrideWithValue(_EmptyAssets()),
          albumRepositoryProvider.overrideWithValue(_EmptyAlbums()),
        ],
      );
      addTearDown(container.dispose);
      container.read(sessionProvider.notifier).state = SessionState(
        session: _session(),
      );

      await t.pumpWidget(
        UncontrolledProviderScope(
          container: container,
          child: const MaterialApp(home: ReviewScreen()),
        ),
      );
      await t.pump();
      await t.pump(const Duration(milliseconds: 100));

      expect(find.byKey(const Key('open-settings-button')), findsOneWidget);
      await t.tap(find.byKey(const Key('open-settings-button')));
      await t.pump();
      await t.pump(const Duration(milliseconds: 100));
      expect(find.text('Settings & account'), findsOneWidget);
      expect(find.text('Delete account'), findsOneWidget);
    },
  );
}
