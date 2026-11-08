import 'package:flutter/foundation.dart';
import 'package:dartastic_opentelemetry/dartastic_opentelemetry.dart';

/// Crash-proof OpenTelemetry facade for the Immich Swipe Android app.
///
/// Telemetry is **off by default**: [init] is a no-op unless the collector
/// endpoint is supplied at build time, because `OTel.initialize()` on its own
/// would default to `http://localhost:4318` and spam a released app.
///
/// Enable it with compile-time defines:
///
/// ```
/// flutter run \
///   --dart-define=OTEL_EXPORTER_OTLP_ENDPOINT=https://collector.example.com:4318 \
///   --dart-define=OTEL_SERVICE_NAME=immich-swipe-mobile \
///   --dart-define=OTEL_EXPORTER_OTLP_HEADERS=authorization=Bearer%20token
/// ```
///
/// Every public helper is a safe no-op while disabled and never throws, so a
/// telemetry problem can never break startup, auth, or the review flow.
class Telemetry {
  Telemetry._();

  static const String _endpoint = String.fromEnvironment(
    'OTEL_EXPORTER_OTLP_ENDPOINT',
  );
  static const String _serviceName = String.fromEnvironment(
    'OTEL_SERVICE_NAME',
    defaultValue: 'immich-swipe-mobile',
  );
  static const String _headersRaw = String.fromEnvironment(
    'OTEL_EXPORTER_OTLP_HEADERS',
  );

  /// Name used for the tracer, logger and meter.
  static const String _instrumentationName = 'immich-swipe';

  static bool _enabled = false;
  static bool _initialized = false;

  static APITracer? _tracer;
  static APILogger? _logger;
  static APIMeter? _meter;

  static BatchSpanProcessor? _spanProcessor;
  static BatchLogRecordProcessor? _logProcessor;

  static APICounter<int>? _swipesCounter;
  static APICounter<int>? _albumAddsCounter;
  static APICounter<int>? _undoCounter;
  static APICounter<int>? _httpRequestsCounter;
  static APICounter<int>? _errorsCounter;
  static APIHistogram<double>? _httpDurationHistogram;

  /// True once [init] configured an exporter successfully.
  static bool get enabled => _enabled;

  /// Reads `key=value,key2=value2` into a header map. Percent-encoded values
  /// (e.g. `Bearer%20token`) are decoded; a malformed pair is skipped.
  static Map<String, String> _parseHeaders(String raw) {
    final headers = <String, String>{};
    for (final part in raw.split(',')) {
      final pair = part.trim();
      if (pair.isEmpty) continue;
      final eq = pair.indexOf('=');
      if (eq <= 0 || eq == pair.length - 1) continue;
      final key = pair.substring(0, eq).trim();
      var value = pair.substring(eq + 1).trim();
      try {
        value = Uri.decodeComponent(value);
      } catch (_) {
        // Keep the raw value when it is not valid percent-encoding.
      }
      if (key.isNotEmpty) headers[key] = value;
    }
    return headers;
  }

  /// Initializes the OTel SDK, exporters and instruments.
  ///
  /// Safe to call exactly once; later calls are ignored. Any failure is
  /// swallowed (logged with [debugPrint]) and leaves telemetry disabled.
  static Future<void> init({String? serviceVersion}) async {
    if (_initialized) return;
    _initialized = true;
    if (_endpoint.isEmpty) {
      // Not configured: stay a complete no-op, never touch OTel.
      _enabled = false;
      return;
    }
    try {
      final headers = _parseHeaders(_headersRaw);

      final spanExporter = OtlpHttpSpanExporter(
        OtlpHttpExporterConfig(
          endpoint: _endpoint,
          headers: headers.isEmpty ? null : headers,
        ),
      );
      _spanProcessor = BatchSpanProcessor(spanExporter);

      final metricExporter = OtlpHttpMetricExporter(
        OtlpHttpMetricExporterConfig(
          endpoint: _endpoint,
          headers: headers.isEmpty ? null : headers,
        ),
      );

      final logExporter = OtlpHttpLogRecordExporter(
        OtlpHttpLogRecordExporterConfig(
          endpoint: _endpoint,
          headers: headers.isEmpty ? null : headers,
        ),
      );
      _logProcessor = BatchLogRecordProcessor(logExporter);

      await OTel.initialize(
        endpoint: _endpoint,
        serviceName: _serviceName,
        serviceVersion: serviceVersion,
        spanProcessor: _spanProcessor,
        metricExporter: metricExporter,
        logRecordProcessor: _logProcessor,
      );

      _tracer = OTel.tracerProvider().getTracer(_instrumentationName);
      _logger = OTel.logger(_instrumentationName);
      _meter = OTel.meterProvider().getMeter(name: _instrumentationName);

      final meter = _meter!;
      _swipesCounter = meter.createCounter<int>(
        name: 'immich_swipe_swipes_total',
        description: 'Swipe decisions made by the user',
      );
      _albumAddsCounter = meter.createCounter<int>(
        name: 'immich_swipe_album_adds_total',
        description: 'Assets successfully added to an album',
      );
      _undoCounter = meter.createCounter<int>(
        name: 'immich_swipe_undo_total',
        description: 'Undo operations',
      );
      _httpRequestsCounter = meter.createCounter<int>(
        name: 'immich_swipe_http_requests_total',
        description: 'HTTP requests issued to the swipe backend',
      );
      _errorsCounter = meter.createCounter<int>(
        name: 'immich_swipe_errors_total',
        description: 'Errors by type',
      );
      _httpDurationHistogram = meter.createHistogram<double>(
        name: 'immich_swipe_http_request_duration_ms',
        unit: 'ms',
        description: 'HTTP request duration in milliseconds',
      );

      _enabled = true;
    } catch (e) {
      _enabled = false;
      _tracer = null;
      _logger = null;
      _meter = null;
      _spanProcessor = null;
      _logProcessor = null;
      debugPrint('Telemetry: initialization failed, staying disabled: $e');
    }
  }

  // ------------------------------------------------------------------
  // Spans
  // ------------------------------------------------------------------

  /// Starts a span, or returns null when telemetry is disabled.
  ///
  /// The span is not made current; use [endSpanOk]/[endSpanError] to finish
  /// it, or [traceAsync] for the auto-managed variant.
  static APISpan? startSpan(
    String name, {
    SpanKind kind = SpanKind.client,
    Map<String, Object>? attributes,
  }) {
    if (!_enabled) return null;
    try {
      final tracer = _tracer;
      if (tracer == null) return null;
      final attrs = <String, Object>{...?attributes};
      return tracer.startSpan(
        name,
        kind: kind,
        attributes: attrs.isEmpty ? null : OTel.attributesFromMap(attrs),
      );
    } catch (_) {
      return null;
    }
  }

  /// Adds attributes to [span] (no-op for null spans).
  static void addSpanAttributes(APISpan? span, Map<String, Object> attributes) {
    if (!_enabled || span == null || attributes.isEmpty) return;
    try {
      span.addAttributes(OTel.attributesFromMap(attributes));
    } catch (_) {}
  }

  /// Ends [span] with `Ok` status.
  static void endSpanOk(APISpan? span) {
    if (span == null) return;
    try {
      span.setStatus(SpanStatusCode.Ok);
    } catch (_) {}
    try {
      span.end();
    } catch (_) {}
  }

  /// Ends [span] with an error status and a recorded exception event.
  static void endSpanError(APISpan? span, Object error, [StackTrace? st]) {
    if (span == null) return;
    try {
      span.setStatus(SpanStatusCode.Error, error.toString());
    } catch (_) {}
    try {
      span.recordException(error, stackTrace: st);
    } catch (_) {}
    try {
      span.end();
    } catch (_) {}
  }

  /// Builds a W3C `traceparent` header for [span], or null when it cannot be
  /// produced (disabled, null span, or all-zero ids).
  static String? traceparentFor(APISpan? span) {
    if (!_enabled || span == null) return null;
    try {
      final ctx = span.spanContext;
      if (!ctx.isValid) return null;
      final traceId = ctx.traceId.hexString;
      final spanId = ctx.spanId.hexString;
      if (traceId == '00000000000000000000000000000000') return null;
      if (spanId == '0000000000000000') return null;
      final flags = ctx.traceFlags.toString();
      return '00-$traceId-$spanId-$flags';
    } catch (_) {
      return null;
    }
  }

  /// Runs [body] inside a CLIENT span, recording status and exceptions.
  ///
  /// Returns whatever [body] returns. Exceptions are recorded and rethrown.
  static Future<T?> traceAsync<T>(
    String name,
    Future<T> Function() body, {
    Map<String, Object>? attributes,
    SpanKind kind = SpanKind.client,
  }) async {
    final span = startSpan(name, kind: kind, attributes: attributes);
    if (span == null) {
      return await body();
    }
    try {
      final result = await body();
      endSpanOk(span);
      return result;
    } catch (e, st) {
      endSpanError(span, e, st);
      rethrow;
    }
  }

  // ------------------------------------------------------------------
  // Counters / histograms
  // ------------------------------------------------------------------

  static void _add(APICounter<int>? counter, [Map<String, Object>? attrs]) {
    if (!_enabled || counter == null) return;
    try {
      counter.add(
        1,
        attrs == null || attrs.isEmpty ? null : OTel.attributesFromMap(attrs),
      );
    } catch (_) {}
  }

  /// Counts a swipe decision. [action] is `keep` or `trash`.
  static void countSwipe(String action) {
    _add(_swipesCounter, {'action': action});
  }

  /// Counts an asset successfully added to an album.
  static void countAlbumAdd() => _add(_albumAddsCounter);

  /// Counts an undo.
  static void countUndo() => _add(_undoCounter);

  /// Counts an error by [type] (e.g. `trash_failed`).
  static void countError(String type) {
    _add(_errorsCounter, {'error.type': type});
  }

  /// Records an HTTP request: duration histogram plus the request counter.
  static void recordHttpDuration(
    num ms, {
    required String method,
    required int? statusCode,
  }) {
    if (!_enabled) return;
    final attributes = <String, Object>{
      'http.request.method': method,
      'http.response.status_code': ?statusCode,
    };
    try {
      _httpDurationHistogram?.record(
        ms.toDouble(),
        OTel.attributesFromMap(<String, Object>{
          'http.request.method': method,
          'http.response.status_code': ?statusCode,
        }),
      );
    } catch (_) {}
    _add(_httpRequestsCounter, attributes);
  }

  // ------------------------------------------------------------------
  // Logs
  // ------------------------------------------------------------------

  static void _emit(
    Severity severity,
    String message,
    Map<String, Object>? attrs,
  ) {
    if (!_enabled) return;
    try {
      final logger = _logger;
      if (logger == null) return;
      logger.emit(
        severityNumber: severity,
        body: message,
        attributes: OTel.attributesFromMap(attrs ?? const <String, Object>{}),
      );
    } catch (_) {}
  }

  /// Emits an INFO log record.
  static void logInfo(String message, [Map<String, Object>? attrs]) =>
      _emit(Severity.INFO, message, attrs);

  /// Emits a WARN log record.
  static void logWarn(String message, [Map<String, Object>? attrs]) =>
      _emit(Severity.WARN, message, attrs);

  /// Emits an ERROR log record, attaching [error]/[stackTrace] when given.
  static void logError(
    String message, {
    Object? error,
    StackTrace? stackTrace,
    Map<String, Object>? attrs,
  }) {
    final merged = <String, Object>{
      ...?attrs,
      if (error != null) 'exception.type': error.runtimeType.toString(),
      if (error != null) 'exception.message': error.toString(),
      if (stackTrace != null) 'exception.stacktrace': stackTrace.toString(),
    };
    _emit(Severity.ERROR, message, merged);
  }

  // ------------------------------------------------------------------
  // Lifecycle
  // ------------------------------------------------------------------

  /// Forces the batch processors and metric reader to export pending data.
  ///
  /// Call when the app is backgrounded: the batch exporters are timer-driven
  /// and those timers do not fire while the process is paused.
  static Future<void> flush() async {
    if (!_enabled) return;
    try {
      await _spanProcessor?.forceFlush();
    } catch (_) {}
    try {
      await OTel.meterProvider().forceFlush();
    } catch (_) {}
    try {
      await _logProcessor?.forceFlush();
    } catch (_) {}
  }

  /// Shuts the SDK down. Telemetry becomes a no-op afterwards.
  static Future<void> shutdown() async {
    if (!_enabled) return;
    try {
      await OTel.shutdown();
    } catch (e) {
      debugPrint('Telemetry: shutdown failed: $e');
    }
    _enabled = false;
  }
}

/// Handle for a manually managed span (used by the HTTP client), so callers
/// never need to import the OpenTelemetry package themselves.
///
/// Behaves as a no-op placeholder when telemetry is disabled.
class TelemetrySpan {
  TelemetrySpan._(this._span);

  final APISpan? _span;
  final Stopwatch _stopwatch = Stopwatch();

  /// Starts a span with [name] and begins timing it.
  static TelemetrySpan start(
    String name, {
    Map<String, Object>? attributes,
    SpanKind kind = SpanKind.client,
  }) {
    final handle = TelemetrySpan._(
      Telemetry.startSpan(name, kind: kind, attributes: attributes),
    );
    handle._stopwatch.start();
    return handle;
  }

  /// W3C `traceparent` value for this span, or null when unavailable.
  String? get traceparent => Telemetry.traceparentFor(_span);

  /// Milliseconds elapsed since [start].
  double get elapsedMs => _stopwatch.elapsedMicroseconds / 1000.0;

  /// Ends the span with `Ok` status after adding [attributes].
  void finishOk({Map<String, Object>? attributes}) {
    Telemetry.addSpanAttributes(_span, attributes ?? const <String, Object>{});
    Telemetry.endSpanOk(_span);
  }

  /// Ends the span with an error status and a recorded exception event.
  void finishError(
    Object error,
    StackTrace stackTrace, {
    Map<String, Object>? attributes,
  }) {
    Telemetry.addSpanAttributes(_span, attributes ?? const <String, Object>{});
    Telemetry.endSpanError(_span, error, stackTrace);
  }
}
