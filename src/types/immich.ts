export interface ImmichAsset {
  id: string
  ownerId: string
  type: 'IMAGE' | 'VIDEO' | 'AUDIO' | 'OTHER'
  originalPath: string
  originalFileName: string
  originalMimeType?: string
  thumbhash?: string
  fileCreatedAt: string
  fileModifiedAt: string
  localDateTime: string
  updatedAt: string
  isFavorite: boolean
  isArchived: boolean
  isTrashed: boolean
  isOffline: boolean
  hasMetadata: boolean
  isEdited?: boolean
  duration?: number | null
  checksum?: string
  visibility?: string
  stack?: unknown
  people?: unknown[]
  tags?: unknown[]
  duplicateId?: string
  exifInfo?: {
    city?: string
    country?: string
    dateTimeOriginal?: string
    description?: string
    exifImageHeight?: number
    exifImageWidth?: number
    make?: string
    model?: string
  }
}

export interface ImmichAlbum {
  id: string
  albumName: string
  assetCount?: number
  createdAt?: string
  updatedAt?: string
  albumThumbnailAssetId?: string
}

export interface ImmichPerson {
  id: string
  name: string
  thumbnailPath?: string | null
  isHidden?: boolean
}

/**
 * The active review-feed scope. Exactly one dimension is selected at a time;
 * `library` means the unfiltered feed. Date ranges use local `from`/`to`
 * dates, mapped to Immich's `takenAfter`/`takenBefore` filter fields when
 * building search requests.
 */
export type ReviewScope =
  | { kind: 'library' }
  | { kind: 'album'; albumId: string }
  | { kind: 'dateRange'; from: string; to: string }
  | { kind: 'favorites' }
  | { kind: 'duplicates' }
  | { kind: 'location'; country?: string; city?: string; state?: string }
  | { kind: 'camera'; make?: string; model?: string }

export interface MetadataSearchRequest {
  page?: number
  size?: number
  order?: 'asc' | 'desc'
  type?: 'IMAGE' | 'VIDEO'
  albumId?: string
  isFavorite?: boolean
  takenAfter?: string
  takenBefore?: string
  personIds?: string[]
  city?: string
  state?: string
  country?: string
  make?: string
  model?: string
}

export interface MetadataSearchResponse {
  assets?: {
    total?: number
    count?: number
    items: ImmichAsset[]
    nextPage?: string | null
  }
}
