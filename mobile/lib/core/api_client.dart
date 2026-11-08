import 'package:dio/dio.dart';

import 'telemetry.dart';

/// Dio client configured for the immich-swipe Go backend.
/// Base URL is `<serverUrl>/api` and every request carries
/// `Authorization: Bearer <swipe-session-token>`.
///
/// The Go proxy strips that header before forwarding and injects
/// Immich credentials (x-api-key or Bearer) server-side.
class ApiClient {
  ApiClient({required String serverUrl, required String? token})
    : _dio = Dio(
        BaseOptions(
          baseUrl: '${serverUrl.replaceAll(RegExp(r'/+$'), '')}/api',
          connectTimeout: const Duration(seconds: 15),
          receiveTimeout: const Duration(seconds: 30),
          headers: {'Accept': 'application/json'},
        ),
      ) {
    _dio.interceptors.add(
      InterceptorsWrapper(
        onRequest: (options, handler) {
          if (token != null && token.isNotEmpty) {
            options.headers['Authorization'] = 'Bearer $token';
          }
          // Ensure JSON content-type when body is present.
          if (options.data != null) {
            options.headers['Content-Type'] = 'application/json';
          }
          _startTelemetry(options);
          handler.next(options);
        },
        onResponse: (response, handler) {
          _finishTelemetryOk(response);
          handler.next(response);
        },
        onError: (err, handler) {
          _finishTelemetryError(err);
          handler.next(err);
        },
      ),
    );
  }

  final Dio _dio;
  Dio get dio => _dio;

  static const String _spanKey = 'otel_span';

  static void _startTelemetry(RequestOptions options) {
    if (!Telemetry.enabled) return;
    try {
      final uri = options.uri;
      final span = TelemetrySpan.start(
        '${options.method} ${uri.path}',
        attributes: {
          'http.request.method': options.method,
          'url.full': uri.toString(),
          'url.path': uri.path,
          'server.address': uri.host,
        },
      );
      final traceparent = span.traceparent;
      if (traceparent != null) {
        options.headers['traceparent'] = traceparent;
      }
      options.extra[_spanKey] = span;
    } catch (_) {
      // Telemetry must never interfere with the request.
    }
  }

  static void _finishTelemetryOk(Response<dynamic> response) {
    final extra = response.requestOptions.extra;
    final span = extra[_spanKey];
    if (span is! TelemetrySpan) return;
    extra.remove(_spanKey);
    final method = response.requestOptions.method;
    final status = response.statusCode;
    try {
      span.finishOk(attributes: {'http.response.status_code': ?status});
      Telemetry.recordHttpDuration(
        span.elapsedMs,
        method: method,
        statusCode: status,
      );
    } catch (_) {}
  }

  static void _finishTelemetryError(DioException err) {
    final extra = err.requestOptions.extra;
    final span = extra[_spanKey];
    if (span is! TelemetrySpan) return;
    extra.remove(_spanKey);
    final method = err.requestOptions.method;
    final status = err.response?.statusCode;
    try {
      span.finishError(
        err,
        StackTrace.current,
        attributes: {
          'http.response.status_code': ?status,
          'error.type': err.type.name,
        },
      );
      Telemetry.recordHttpDuration(
        span.elapsedMs,
        method: method,
        statusCode: status,
      );
      Telemetry.countError('http_${err.type.name}');
    } catch (_) {}
  }

  /// Normalise Dio error into a typed exception for repos to map.
  static AppException mapError(DioException e) {
    final status = e.response?.statusCode;
    final data = e.response?.data;
    String code = '';
    String message = e.message ?? 'Network error';
    if (data is Map) {
      code = (data['code'] ?? data['error'] ?? '').toString();
      message = (data['message'] ?? data['error'] ?? message).toString();
    } else if (data is String && data.isNotEmpty) {
      try {
        // Try to avoid showing raw HTML
        message = data.length > 500 ? data.substring(0, 500) : data;
      } catch (_) {}
    }
    if (status == 401) {
      return AppException('unauthorized', message, statusCode: 401);
    }
    if (status == 428) {
      return AppException('api_key_required', message, statusCode: 428);
    }
    if (code.isNotEmpty) return AppException(code, message, statusCode: status);
    return AppException('network_error', message, statusCode: status);
  }
}

class AppException implements Exception {
  AppException(this.code, this.message, {this.statusCode});
  final String code;
  final String message;
  final int? statusCode;
  @override
  String toString() => 'AppException($code): $message';
}
