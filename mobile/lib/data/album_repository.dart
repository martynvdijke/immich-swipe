import 'package:dio/dio.dart';
import '../core/api_client.dart';
import '../models/immich_album.dart';

class AlbumRepository {
  AlbumRepository(this._dio);
  final Dio _dio;

  Future<List<ImmichAlbum>> listAlbums() async {
    try {
      final results = await Future.wait([
        _dio.get('/albums'),
        _dio.get('/albums', queryParameters: {'isShared': 'true'}),
      ]);
      final a = results[0];
      final b = results[1];
      final map = <String, ImmichAlbum>{};
      for (final src in [a.data, b.data]) {
        if (src is List) {
          for (final e in src) {
            final al = ImmichAlbum.fromJson(e as Map<String, dynamic>);
            map[al.id] = al;
          }
        }
      }
      return map.values.toList();
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }

  Future<void> addToAlbum(String albumId, String assetId) async {
    try {
      await _dio.put('/albums/$albumId/assets', data: {'ids': [assetId]});
    } on DioException catch (e) {
      throw ApiClient.mapError(e);
    }
  }
}
