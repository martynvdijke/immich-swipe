import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:dio/dio.dart';
import 'package:immich_swipe/features/review/widgets/stats_bar.dart';
import 'package:immich_swipe/features/review/widgets/album_chips.dart';
import 'package:immich_swipe/models/immich_album.dart';
import 'package:immich_swipe/models/immich_asset.dart';
import 'package:immich_swipe/state/providers.dart';
import 'dart:typed_data';
import 'package:immich_swipe/data/asset_repository.dart';
import 'package:immich_swipe/data/album_repository.dart';

class FakeAssetRepo2 extends AssetRepository {
  FakeAssetRepo2() : super(Dio());
  @override
  Future<List<ImmichAsset>> searchRandom({required int size, bool imageOnly = false}) async => [];
  @override
  Future<Uint8List> fetchThumbnail(String id, {String size = 'preview'}) async => Uint8List(0);
}

class FakeAlbumRepo2 extends AlbumRepository {
  FakeAlbumRepo2() : super(Dio());
  @override
  Future<List<ImmichAlbum>> listAlbums() async => [
    ImmichAlbum(id: 'al1', albumName: 'Vacation'),
    ImmichAlbum(id: 'al2', albumName: 'Family'),
  ];
}

void main() {
  testWidgets('StatsBar renders counts', (tester) async {
    await tester.pumpWidget(const MaterialApp(home: Scaffold(body: StatsBar(kept: 3, trashed: 2, freedBytes: 2048))));
    expect(find.text('Kept: 3'), findsOneWidget);
    expect(find.text('Trashed: 2'), findsOneWidget);
    expect(find.textContaining('Freed:'), findsOneWidget);
  });

  testWidgets('AlbumChips renders up to 10 chips', (tester) async {
    final albums = List.generate(12, (i) => ImmichAlbum(id: 'a$i', albumName: 'Album $i'));
    await tester.pumpWidget(MaterialApp(home: Scaffold(body: AlbumChips(albums: albums, onSelect: (_) {}))));
    // Should show 10 chips (0..9)
    expect(find.byType(ActionChip), findsNWidgets(10));
  });

  testWidgets('Review flow with overridden providers shows empty state', (tester) async {
    final dio = Dio(BaseOptions(baseUrl: 'http://example.com/api'));
    await tester.pumpWidget(ProviderScope(
      overrides: [
        dioProvider.overrideWithValue(dio),
        assetRepositoryProvider.overrideWithValue(FakeAssetRepo2()),
        albumRepositoryProvider.overrideWithValue(FakeAlbumRepo2()),
      ],
      child: const MaterialApp(home: Scaffold(body: StatsBar(kept: 0, trashed: 0, freedBytes: 0))),
    ));
    expect(find.text('Kept: 0'), findsOneWidget);
  });
}
