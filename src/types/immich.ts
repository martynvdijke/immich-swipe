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

export interface MetadataSearchRequest {
  page?: number
  size?: number
  order?: 'asc' | 'desc'
  type?: 'IMAGE' | 'VIDEO'
}

export interface MetadataSearchResponse {
  assets?: {
    total?: number
    count?: number
    items: ImmichAsset[]
    nextPage?: string | null
  }
}
