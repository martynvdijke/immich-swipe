import 'dart:async';
import 'dart:convert';
import 'dart:math';

import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';

/// Compile-time telemetry configuration.
///
/// These values are only fallbacks: the mobile app normally fetches the
/// collector endpoint at runtime from `GET /api/otel-settings`, so a released
/// build follows whatever the server it talks to is configured with. Build-time
/// defines still work when the server does not expose the endpoint.
abstract final class TelemetryConfig {
  static const otelEndpoint = String.fromEnvironment(
    'OTEL_EXPORTER_OTLP_ENDPOINT',
  );
  static const otelHeaders = String.fromEnvironment(
    'OTEL_EXPORTER_OTLP_HEADERS',
  );
  static const serviceName = String.fromEnvironment(
    'OTEL_SERVICE_NAME',
    defaultValue: 'immich-swipe-android',
  );
  static const serviceVersion = String.fromEnvironment(
    'OTEL_SERVICE_VERSION',
    defaultValue: '1.0.0',
  );
  static const umamiUrl = String.fromEnvironment('UMAMI_URL');
  static const umamiWebsiteId = String.fromEnvironment('UMAMI_WEBSITE_ID');

  static bool get otelEnabled => otelEndpoint.trim().isNotEmpty && !kIsWeb;
  static bool get umamiEnabled =>
      !kIsWeb && umamiUrl.trim().isNotEmpty && umamiWebsiteId.trim().isNotEmpty;
}

/// Server-provided OpenTelemetry configuration.
///
/// Parsed leniently because the endpoint is also served by older/newer
/// deployments: both `endpoint`/`otel_endpoint` and `enabled`/`otel_enabled`
/// spellings are accepted, and a missing `traces_enabled` inherits `enabled`.
class OtelSettings {
  const OtelSettings({
    this.endpoint = '',
    this.enabled = false,
    this.tracesEnabled = false,
    this.metricsEnabled = false,
    this.logsEnabled = false,
  });

  final String endpoint;
  final bool enabled;
  final bool tracesEnabled;
  final bool metricsEnabled;
  final bool logsEnabled;

  /// True when spans should actually be exported.
  bool get isActive =>
      enabled && tracesEnabled && endpoint.trim().isNotEmpty && !kIsWeb;

  factory OtelSettings.fromJson(Map<String, dynamic> json) {
    String readString(List<String> keys) {
      for (final key in keys) {
        final value = json[key];
        if (value == null) continue;
        final text = value.toString().trim();
        if (text.isNotEmpty) return text;
      }
      return '';
    }

    bool readBool(List<String> keys, bool fallback) {
      for (final key in keys) {
        if (!json.containsKey(key)) continue;
        final value = json[key];
        if (value is bool) return value;
        if (value is num) return value != 0;
        if (value is String) {
          switch (value.trim().toLowerCase()) {
            case 'true':
            case '1':
            case 'yes':
            case 'on':
              return true;
            case 'false':
            case '0':
            case 'no':
            case 'off':
              return false;
          }
        }
      }
      return fallback;
    }

    final endpoint = readString(const ['endpoint', 'otel_endpoint']);
    final hasEndpoint = endpoint.isNotEmpty;
    final enabled = readBool(const ['enabled', 'otel_enabled'], hasEndpoint);
    final traces = readBool(const ['traces_enabled', 'otel_traces_enabled'], enabled);
    final metrics = readBool(const ['metrics_enabled', 'otel_metrics_enabled'], false);
    final logs = readBool(const ['logs_enabled', 'otel_logs_enabled'], false);

    if (!hasEndpoint) {
      return const OtelSettings();
    }
    return OtelSettings(
      endpoint: endpoint,
      enabled: enabled,
      tracesEnabled: traces,
      metricsEnabled: metrics,
      logsEnabled: logs,
    );
  }
}

/// Fetches the runtime telemetry configuration from the swipe backend.
///
/// Returns null on any error, timeout or non-200 response and never throws, so
/// startup can safely ignore the result while offline.
Future<OtelSettings?> fetchOtelSettings(
  String baseUrl, {
  Dio? dio,
  Duration timeout = const Duration(seconds: 3),
}) async {
  final client = dio ?? Dio();
  try {
    final base = baseUrl.trim().replaceAll(RegExp(r'/+$'), '');
    if (base.isEmpty) return null;
    final response = await client
        .get<dynamic>(
          '$base/api/otel-settings',
          options: Options(
            responseType: ResponseType.json,
            sendTimeout: timeout,
            receiveTimeout: timeout,
          ),
        )
        .timeout(timeout);
    if (response.statusCode != 200) return null;
    final data = response.data;
    if (data is Map) {
      return OtelSettings.fromJson(Map<String, dynamic>.from(data));
    }
    if (data is String && data.trim().isNotEmpty) {
      final decoded = jsonDecode(data);
      if (decoded is Map) {
        return OtelSettings.fromJson(Map<String, dynamic>.from(decoded));
      }
    }
    return null;
  } catch (_) {
    return null;
  }
}

String randomHex(int bytes) {
  final random = Random.secure();
  final buffer = StringBuffer();
  for (var i = 0; i < bytes; i++) {
    buffer.write(random.nextInt(256).toRadixString(16).padLeft(2, '0'));
  }
  return buffer.toString();
}

String buildTraceparent(String traceId, String spanId) =>
    '00-$traceId-$spanId-01';

class TraceSpan {
  TraceSpan({
    required this.name,
    required this.attributes,
    required this.traceId,
    required this.spanId,
    required this.startNanos,
    this.kind = 1,
  });

  final String name;
  final Map<String, Object> attributes;
  final String traceId;
  final String spanId;
  final int startNanos;
  final int kind;
  bool _ended = false;
  int? statusCode;
  String error = '';

  String get traceparent => buildTraceparent(traceId, spanId);

  void end({
    int? httpStatus,
    Object? failure,
    Map<String, Object> extra = const {},
  }) {
    if (_ended) return;
    _ended = true;
    statusCode = httpStatus;
    if (failure != null) error = failure.toString();
    final attrs = <String, Object>{...attributes, ...extra};
    if (httpStatus != null) attrs['http.response.status_code'] = httpStatus;
    if (error.isNotEmpty) attrs['error.message'] = _short(error);
    Telemetry.instance._finish(
      this,
      attrs,
      error.isNotEmpty || (httpStatus != null && httpStatus >= 500),
    );
  }

  String _short(String value) =>
      value.length > 200 ? '${value.substring(0, 200)}…' : value;
}

/// Builds an OTLP/HTTP JSON trace payload for a single span.
Map<String, Object?> buildOtlpPayload({
  required String serviceName,
  required String serviceVersion,
  required TraceSpan span,
  required Map<String, Object> attributes,
  required bool failed,
  required int endNanos,
}) {
  List<Map<String, Object?>> attrs(Map<String, Object> values) => values.entries
      .map(
        (entry) => {
          'key': entry.key,
          'value': switch (entry.value) {
            final bool value => {'boolValue': value},
            final int value => {'intValue': value.toString()},
            final double value => {'doubleValue': value},
            _ => {'stringValue': entry.value.toString()},
          },
        },
      )
      .toList();

  return {
    'resourceSpans': [
      {
        'resource': {
          'attributes': attrs({
            'service.name': serviceName,
            'service.version': serviceVersion,
            'telemetry.sdk.name': 'immich-swipe-mobile',
            'telemetry.sdk.language': 'dart',
          }),
        },
        'scopeSpans': [
          {
            'scope': {'name': 'immich-swipe.mobile', 'version': serviceVersion},
            'spans': [
              {
                'traceId': span.traceId,
                'spanId': span.spanId,
                'name': span.name,
                'kind': span.kind,
                'startTimeUnixNano': span.startNanos.toString(),
                'endTimeUnixNano': endNanos.toString(),
                'attributes': attrs(attributes),
                'status': {'code': failed ? 2 : 1},
              },
            ],
          },
        ],
      },
    ],
  };
}

/// Hand-rolled OTLP/HTTP traces exporter.
///
/// Deliberately dependency-light: the SDK package is not used, so the exporter
/// builds the OTLP JSON itself and posts it with [Dio]. Every public method is
/// a safe no-op while telemetry is disabled and never throws.
class Telemetry {
  Telemetry._();

  static final Telemetry instance = Telemetry._();

  Dio? _dio;
  Timer? _timer;
  final List<Map<String, Object?>> _queue = [];
  bool _started = false;
  bool _bootstrapAttempted = false;
  String _tracesEndpoint = '';
  Map<String, String> _headers = const {};

  bool get otelEnabled => _tracesEndpoint.isNotEmpty;
  bool get umamiEnabled => TelemetryConfig.umamiEnabled;

  /// True once an exporter endpoint is configured.
  static bool get enabled => instance.otelEnabled;

  /// Starts exporting spans (idempotent, and a no-op on web).
  Future<void> start({String? endpoint, Map<String, String>? headers}) async {
    if (_started || kIsWeb) return;
    final raw = (endpoint ?? TelemetryConfig.otelEndpoint).trim();
    if (raw.isEmpty) return;
    _started = true;
    _dio = Dio(
      BaseOptions(
        connectTimeout: const Duration(seconds: 5),
        sendTimeout: const Duration(seconds: 10),
        receiveTimeout: const Duration(seconds: 10),
      ),
    );
    _tracesEndpoint = normalizeEndpoint(raw);
    _headers =
        headers ?? parseOtelHeaders(TelemetryConfig.otelHeaders);
    _timer ??= Timer.periodic(const Duration(seconds: 5), (_) => _flush());
    beginSpan('app.start').end();
  }

  /// Resolves the collector endpoint from the backend, then starts exporting.
  ///
  /// Falls back to the compile-time endpoint when the request fails (offline,
  /// old server, or telemetry disabled server-side). Never throws.
  Future<void> startFromServer(String baseUrl) async {
    if (_started || _bootstrapAttempted || kIsWeb) return;
    _bootstrapAttempted = true;
    OtelSettings? settings;
    try {
      settings = await fetchOtelSettings(baseUrl);
    } catch (_) {
      settings = null;
    }
    if (settings != null && settings.isActive) {
      try {
        await start(endpoint: settings.endpoint);
      } catch (_) {}
      return;
    }
    try {
      await start();
    } catch (_) {}
  }

  TraceSpan beginSpan(
    String name, {
    Map<String, Object> attributes = const {},
    int kind = 1,
  }) {
    return TraceSpan(
      name: name,
      attributes: attributes,
      traceId: randomHex(16),
      spanId: randomHex(8),
      startNanos: DateTime.now().microsecondsSinceEpoch * 1000,
      kind: kind,
    );
  }

  void screen(String name) {
    beginSpan('screen $name', attributes: {'screen.name': name}).end();
    track('screen_view', {'screen': name});
  }

  /// Sends a small, allowlisted analytics event to Umami when configured.
  void track(String event, [Map<String, Object?> props = const {}]) {
    if (!TelemetryConfig.umamiEnabled) return;
    final data = <String, Object>{};
    for (final entry in props.entries) {
      final value = entry.value;
      if (value is bool || value is num) {
        data[entry.key] = value as Object;
      } else if (value is String && value.length <= 64) {
        data[entry.key] = value;
      }
    }
    final base = TelemetryConfig.umamiUrl.replaceAll(RegExp(r'/+$'), '');
    final uri = Uri.tryParse('$base/api/send');
    if (uri == null) return;
    final client = _dio ?? Dio();
    unawaited(
      client
          .post<dynamic>(
            uri.toString(),
            data: {
              'type': 'event',
              'payload': {
                'website': TelemetryConfig.umamiWebsiteId,
                'hostname': 'immich-swipe.app',
                'url': '/app/$event',
                'event': event,
                'data': data,
              },
            },
            options: Options(contentType: Headers.jsonContentType),
          )
          .then((_) {}, onError: (Object _) {}),
    );
  }

  void _finish(TraceSpan span, Map<String, Object> attributes, bool failed) {
    if (!otelEnabled) return;
    final payload = buildOtlpPayload(
      serviceName: TelemetryConfig.serviceName,
      serviceVersion: TelemetryConfig.serviceVersion,
      span: span,
      attributes: attributes,
      failed: failed,
      endNanos: DateTime.now().microsecondsSinceEpoch * 1000,
    );
    _queue.add(payload);
    if (_queue.length >= 20) _flush();
  }

  void _flush() {
    if (_queue.isEmpty || _dio == null || _tracesEndpoint.isEmpty) return;
    final spans = <Map<String, Object?>>[];
    for (final payload in _queue) {
      final resourceSpans = payload['resourceSpans'];
      if (resourceSpans is List && resourceSpans.isNotEmpty) {
        final resource = resourceSpans.first;
        if (resource is Map) {
          final scopeSpans = resource['scopeSpans'];
          if (scopeSpans is List && scopeSpans.isNotEmpty) {
            final scope = scopeSpans.first;
            if (scope is Map && scope['spans'] is List) {
              spans.addAll(
                (scope['spans'] as List).cast<Map<String, Object?>>(),
              );
            }
          }
        }
      }
    }
    _queue.clear();
    if (spans.isEmpty) return;
    final payload = {
      'resourceSpans': [
        {
          'resource': {
            'attributes': [
              {
                'key': 'service.name',
                'value': {'stringValue': TelemetryConfig.serviceName},
              },
              {
                'key': 'service.version',
                'value': {'stringValue': TelemetryConfig.serviceVersion},
              },
            ],
          },
          'scopeSpans': [
            {
              'scope': {'name': 'immich-swipe.mobile'},
              'spans': spans,
            },
          ],
        },
      ],
    };
    unawaited(
      _dio!
          .post<dynamic>(
            _tracesEndpoint,
            data: payload,
            options: Options(
              contentType: Headers.jsonContentType,
              headers: _headers.isEmpty ? null : _headers,
            ),
          )
          .then(
            (_) {},
            onError: (Object error) {
              if (kDebugMode) debugPrint('OTLP export failed: $error');
            },
          ),
    );
  }

  /// Exports queued spans now (call when the app is backgrounded).
  void flush() => _flush();

  Future<void> shutdown() async {
    _timer?.cancel();
    _timer = null;
    _flush();
    await Future<void>.delayed(const Duration(milliseconds: 200));
    _dio?.close(force: true);
    _dio = null;
    _started = false;
  }

  // ------------------------------------------------------------------
  // Static façade used by app code. Counters become lightweight spans so
  // the exported traces still carry the signal without an extra exporter.
  // ------------------------------------------------------------------

  static void countSwipe(String action) => _emit(
    'swipe',
    {'action': action},
  );

  static void countAlbumAdd() => _emit('album.add.count');

  static void countUndo() => _emit('swipe.undo');

  static void countError(String type) =>
      _emit('app.error', {'error.type': type});

  static void recordHttpDuration(
    num ms, {
    required String method,
    required int? statusCode,
  }) => _emit('http.client', {
    'http.request.method': method,
    'http.response.status_code': ?statusCode,
    'http.client.duration_ms': ms.round(),
  });

  static Future<T?> traceAsync<T>(
    String name,
    Future<T> Function() body, {
    Map<String, Object>? attributes,
  }) async {
    final span = instance.beginSpan(name, attributes: {...?attributes});
    try {
      final result = await body();
      span.end();
      return result;
    } catch (error) {
      span.end(failure: error);
      rethrow;
    }
  }

  static void _emit(String name, [Map<String, Object> attributes = const {}]) {
    if (!enabled) return;
    instance.beginSpan(name, attributes: attributes).end();
  }

  /// Ensures a raw endpoint points at the OTLP traces path.
  static String normalizeEndpoint(String raw) {
    final base = raw.trim().replaceAll(RegExp(r'/+$'), '');
    if (base.endsWith('/v1/traces')) return base;
    return '$base/v1/traces';
  }

  /// Parses `key=value,key2=value2` headers, decoding percent-encoded values.
  static Map<String, String> parseOtelHeaders(String raw) {
    final out = <String, String>{};
    for (final pair in raw.split(',')) {
      final idx = pair.indexOf('=');
      if (idx <= 0) continue;
      final key = pair.substring(0, idx).trim();
      var value = pair.substring(idx + 1).trim();
      try {
        value = Uri.decodeComponent(value);
      } catch (_) {}
      if (key.isNotEmpty && value.isNotEmpty) out[key] = value;
    }
    return out;
  }
}

/// Handle for a manually managed span (used by the HTTP client), so callers
/// never need to know the exporter internals. A no-op when disabled.
class TelemetrySpan {
  TelemetrySpan._(this._span);

  final TraceSpan? _span;
  final Stopwatch _stopwatch = Stopwatch();

  static TelemetrySpan start(
    String name, {
    Map<String, Object>? attributes,
  }) {
    final handle = TelemetrySpan._(
      Telemetry.enabled
          ? Telemetry.instance.beginSpan(
              name,
              attributes: {...?attributes},
            )
          : null,
    );
    handle._stopwatch.start();
    return handle;
  }

  String? get traceparent => _span?.traceparent;

  double get elapsedMs => _stopwatch.elapsedMicroseconds / 1000.0;

  void finishOk({Map<String, Object>? attributes}) {
    _span?.end(
      httpStatus: switch (attributes?['http.response.status_code']) {
        final int value => value,
        _ => null,
      },
      extra: {...?attributes},
    );
  }

  void finishError(
    Object error,
    StackTrace stackTrace, {
    Map<String, Object>? attributes,
  }) {
    _span?.end(
      httpStatus: switch (attributes?['http.response.status_code']) {
        final int value => value,
        _ => null,
      },
      failure: error,
      extra: {...?attributes},
    );
  }
}
