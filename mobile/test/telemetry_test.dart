import 'dart:convert';
import 'dart:typed_data';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:immich_swipe/src/telemetry/telemetry.dart';

/// Minimal Dio adapter so no real HTTP leaves the test process.
class _FakeAdapter implements HttpClientAdapter {
  _FakeAdapter(this._handler);

  final Future<ResponseBody> Function(RequestOptions options) _handler;

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) => _handler(options);

  @override
  void close({bool force = false}) {}
}

Dio _dioReturning(
  Object? body, {
  int statusCode = 200,
  bool throwError = false,
}) {
  final dio = Dio(BaseOptions(baseUrl: 'https://immich-swipe.vandijke.xyz'));
  dio.httpClientAdapter = _FakeAdapter((options) async {
    if (throwError) {
      throw DioException.connectionError(
        requestOptions: options,
        reason: 'offline',
      );
    }
    return ResponseBody.fromString(
      body is String ? body : jsonEncode(body),
      statusCode,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  });
  return dio;
}

void main() {
  group('normalizeEndpoint', () {
    test('appends the OTLP traces path', () {
      expect(
        Telemetry.normalizeEndpoint('https://collector.example.com:4318'),
        'https://collector.example.com:4318/v1/traces',
      );
    });

    test('keeps an existing traces path and trims slashes', () {
      expect(
        Telemetry.normalizeEndpoint(' https://collector.example.com/v1/traces/ '),
        'https://collector.example.com/v1/traces',
      );
      expect(
        Telemetry.normalizeEndpoint('https://collector.example.com///'),
        'https://collector.example.com/v1/traces',
      );
    });
  });

  group('parseOtelHeaders', () {
    test('parses pairs and decodes values', () {
      expect(
        Telemetry.parseOtelHeaders(
          'authorization=Bearer%20token, x-scope=team',
        ),
        {'authorization': 'Bearer token', 'x-scope': 'team'},
      );
    });

    test('skips malformed pairs', () {
      expect(Telemetry.parseOtelHeaders('nonsense,=x,key='), isEmpty);
      expect(Telemetry.parseOtelHeaders(''), isEmpty);
    });
  });

  group('buildOtlpPayload', () {
    test('shapes the OTLP JSON envelope', () {
      final span = TraceSpan(
        name: 'GET /api/albums',
        attributes: const {'url.path': '/api/albums'},
        traceId: 'a' * 32,
        spanId: 'b' * 16,
        startNanos: 1700000000000000000,
      );

      final payload = buildOtlpPayload(
        serviceName: 'immich-swipe-android',
        serviceVersion: '9.9.9',
        span: span,
        attributes: {
          ...span.attributes,
          'http.response.status_code': 200,
          'ok': true,
        },
        failed: false,
        endNanos: 1700000001000000000,
      );

      final resourceSpans = (payload['resourceSpans'] as List).single as Map;
      final resource = resourceSpans['resource'] as Map;
      final resourceAttrs = resource['attributes'] as List;
      final names = {
        for (final attr in resourceAttrs)
          (attr as Map)['key']: (attr['value'] as Map).values.first,
      };
      expect(names['service.name'], 'immich-swipe-android');
      expect(names['service.version'], '9.9.9');
      expect(names['telemetry.sdk.name'], 'immich-swipe-mobile');

      final scopeSpans = (resourceSpans['scopeSpans'] as List).single as Map;
      expect((scopeSpans['scope'] as Map)['name'], 'immich-swipe.mobile');

      final outSpan = (scopeSpans['spans'] as List).single as Map;
      expect(outSpan['traceId'], 'a' * 32);
      expect(outSpan['spanId'], 'b' * 16);
      expect(outSpan['name'], 'GET /api/albums');
      expect(outSpan['startTimeUnixNano'], '1700000000000000000');
      expect(outSpan['endTimeUnixNano'], '1700000001000000000');
      expect((outSpan['status'] as Map)['code'], 1);

      final attrMap = {
        for (final attr in outSpan['attributes'] as List)
          (attr as Map)['key']: (attr['value'] as Map).values.first,
      };
      expect(attrMap['http.response.status_code'], '200');
      expect(attrMap['ok'], true);
    });

    test('marks failures with error status', () {
      final span = TraceSpan(
        name: 'GET /api/x',
        attributes: const {},
        traceId: 'c' * 32,
        spanId: 'd' * 16,
        startNanos: 1,
      );
      final payload = buildOtlpPayload(
        serviceName: 'immich-swipe-android',
        serviceVersion: '1.0.0',
        span: span,
        attributes: const {},
        failed: true,
        endNanos: 2,
      );
      final resourceSpans = (payload['resourceSpans'] as List).single as Map;
      final scopeSpans = (resourceSpans['scopeSpans'] as List).single as Map;
      final outSpan = (scopeSpans['spans'] as List).single as Map;
      expect((outSpan['status'] as Map)['code'], 2);
    });
  });

  group('OtelSettings.fromJson', () {
    test('reads endpoint/enabled and defaults traces to enabled', () {
      final settings = OtelSettings.fromJson({
        'endpoint': 'https://collector.example.com:4318',
        'enabled': true,
      });
      expect(settings.endpoint, 'https://collector.example.com:4318');
      expect(settings.enabled, isTrue);
      expect(settings.tracesEnabled, isTrue);
      expect(settings.metricsEnabled, isFalse);
      expect(settings.logsEnabled, isFalse);
      expect(settings.isActive, isTrue);
    });

    test('accepts snake_case aliases and string booleans', () {
      final settings = OtelSettings.fromJson({
        'otel_endpoint': 'https://collector.example.com',
        'otel_enabled': 'true',
        'traces_enabled': 'false',
        'metrics_enabled': 1,
        'logs_enabled': '0',
      });
      expect(settings.endpoint, 'https://collector.example.com');
      expect(settings.enabled, isTrue);
      expect(settings.tracesEnabled, isFalse);
      expect(settings.metricsEnabled, isTrue);
      expect(settings.logsEnabled, isFalse);
      expect(settings.isActive, isFalse);
    });

    test('empty endpoint disables everything', () {
      final settings = OtelSettings.fromJson({'endpoint': '', 'enabled': true});
      expect(settings.endpoint, isEmpty);
      expect(settings.enabled, isFalse);
      expect(settings.tracesEnabled, isFalse);
      expect(settings.isActive, isFalse);
    });

    test('missing fields are tolerated', () {
      final settings = OtelSettings.fromJson({});
      expect(settings.endpoint, isEmpty);
      expect(settings.enabled, isFalse);
      expect(settings.isActive, isFalse);
    });
  });

  group('fetchOtelSettings', () {
    test('parses a 200 response', () async {
      final dio = _dioReturning({
        'endpoint': 'https://collector.example.com:4318',
        'enabled': true,
        'traces_enabled': true,
        'metrics_enabled': false,
        'logs_enabled': false,
      });

      final settings = await fetchOtelSettings(
        'https://immich-swipe.vandijke.xyz/',
        dio: dio,
      );
      expect(settings, isNotNull);
      expect(settings!.endpoint, 'https://collector.example.com:4318');
      expect(settings.isActive, isTrue);
    });

    test('returns null on a non-200 response', () async {
      final dio = _dioReturning({'endpoint': 'x'}, statusCode: 404);
      expect(
        await fetchOtelSettings('https://immich-swipe.vandijke.xyz', dio: dio),
        isNull,
      );
    });

    test('returns null when the request throws', () async {
      final dio = _dioReturning(null, throwError: true);
      expect(
        await fetchOtelSettings('https://immich-swipe.vandijke.xyz', dio: dio),
        isNull,
      );
    });

    test('returns null on an empty base url', () async {
      expect(await fetchOtelSettings('   ', dio: Dio()), isNull);
    });
  });
}
