// Package webpcodec decodes standalone WebP images with pre-allocation limits.
// Animated containers must first be demultiplexed into standalone frames.
//
// The budget covers cumulative decoder-owned backing storage, not just canvas
// pixels: VP8L meta-Huffman images can select up to 65536 groups on a tiny
// canvas. Temporary allocations are never credited back, so admission does not
// depend on when GC reclaims them. The budget includes returned raster storage;
// callers must separately budget encoded input, transforms and composition.
// Fixed scratch is conservatively reserved, but runtime/allocator bookkeeping
// is not a hard RSS bound. Decoding is in-process, without a hard CPU deadline.
//
// DecodeConfig reads headers, not compressed alpha or entropy-coded pixels.
// It intentionally retains upstream semantics and is not an allocation-budget
// check for a subsequent Decode. This package never registers a global decoder.
//
// The VP8L fork and WebP adapter are based on x/image v0.38.0; see NOTICE.
// Re-audit all decoder allocation sites, including VP8's externally maintained
// frame/partition allocations, when upgrading x/image.
package webpcodec
