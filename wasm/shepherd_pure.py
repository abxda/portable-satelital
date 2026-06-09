# shepherd_pure.py — Shepherd segmentation (Shepherd, Bunting & Dymond 2019)
# en numpy/scipy PURO, sin numba. Port FIEL de pyshepseg 2.0.5
# (pyshepseg/shepseg.py, MIT, Flood & Gillingham) — Fase A de la ruta a WASM.
#
# Fidelidad replicada deliberadamente (no "mejorar" sin actualizar el oráculo):
#   * clump por orden de DESCUBRIMIENTO en barrido raster (los IDs gobiernan el
#     orden de eliminación) con tope MAX_CLUMP_SIZE=10000 que parte clumps.
#   * eliminateSinglePixels multi-pasada ANTES de la eliminación general,
#     fusionando con el PÍXEL vecino más parecido (no el segmento).
#   * eliminación iterativa: targetSize 1..minSegmentSize-1, multi-pasada
#     (máx 10), vecino con segSize ESTRICTAMENTE mayor, distancia euclidiana
#     entre MEDIAS espectrales (acumuladas en float32), desempate = primero
#     encontrado en orden píxel→fila→columna del segmento.
#   * segLoc conserva el ORDEN de pixeles (raster; al fusionar: vecino primero)
#     porque el desempate de findMergeSegment depende de ese orden.
#   * relabel idéntico (decremento acumulado por IDs huecos).
#
# Pensado para correr igual en CPython y en Pyodide (numpy 1.x y 2.x).

import numpy as np
from scipy import ndimage

SegIdType = np.uint32
SEGNULLVAL = 0
MINSEGID = SEGNULLVAL + 1
MAX_CLUMP_SIZE = 10000  # mismo tope que pyshepseg


class SegmentationResult:
    """Misma interfaz que pyshepseg.shepseg.SegmentationResult."""

    def __init__(self):
        self.segimg = None
        self.kmeans = None
        self.maxSpectralDiff = None
        self.singlePixelsEliminated = None
        self.smallSegmentsEliminated = None


def doShepherdSegmentation(img, numClusters=60, clusterSubsamplePcnt=1,
        minSegmentSize=50, maxSpectralDiff='auto', imgNullVal=None,
        fourConnected=True, verbose=False, fixedKMeansInit=False,
        kmeansObj=None, spectDistPcntile=50):
    """Misma firma y semántica que pyshepseg.shepseg.doShepherdSegmentation."""
    if kmeansObj is not None:
        km = kmeansObj
    else:
        km = fitSpectralClusters(img, numClusters, clusterSubsamplePcnt,
                                 imgNullVal, fixedKMeansInit)
    clusters = applySpectralClusters(km, img, imgNullVal)

    seg, maxSegId = clump(clusters, SEGNULLVAL, fourConnected, MINSEGID)
    maxSegId = maxSegId - 1
    if verbose:
        print("clumps:", maxSegId)

    segSize = makeSegSize(seg)
    oldMaxSegId = maxSegId
    eliminateSinglePixels(img, seg, segSize, MINSEGID, maxSegId, fourConnected)
    maxSegId = int(seg.max())
    numElimSinglepix = oldMaxSegId - maxSegId
    if verbose:
        print("single pixels eliminados:", numElimSinglepix)

    maxSpectralDiff = autoMaxSpectralDiff(km, maxSpectralDiff, spectDistPcntile)

    numElimSmall = eliminateSmallSegments(seg, img, maxSegId, minSegmentSize,
                                          maxSpectralDiff, fourConnected, MINSEGID)
    if verbose:
        print("segmentos pequeños eliminados:", numElimSmall)

    res = SegmentationResult()
    res.segimg = seg
    res.kmeans = km
    res.maxSpectralDiff = maxSpectralDiff
    res.singlePixelsEliminated = numElimSinglepix
    res.smallSegmentsEliminated = numElimSmall
    return res


# ---------------------------------------------------------------- paso 1

def fitSpectralClusters(img, numClusters, subsamplePcnt, imgNullVal,
                        fixedKMeansInit):
    from sklearn.cluster import KMeans
    nBands, nRows, nCols = img.shape
    xFull = np.transpose(img, (1, 2, 0)).reshape((nRows * nCols, nBands))
    if imgNullVal is not None:
        xFull = xFull[(xFull != imgNullVal).all(axis=1)]
    skip = int(round(100.0 / subsamplePcnt))
    xSample = xFull[::skip]

    numTrials = 5
    init = 'k-means++'
    if fixedKMeansInit:
        init = _diagonalClusterCentres(xSample, numClusters)
        numTrials = 1
    km = KMeans(n_clusters=numClusters, n_init=numTrials, init=init)
    km.fit(xSample)
    return km


def _diagonalClusterCentres(xSample, numClusters):
    bandMin = xSample.min(axis=0)
    bandMax = xSample.max(axis=0)
    step = (bandMax - bandMin) / (numClusters + 1)
    centres = np.empty((numClusters, xSample.shape[1]), dtype=xSample.dtype)
    for i in range(numClusters):
        centres[i] = bandMin + (i + 1) * step
    return centres


def applySpectralClusters(kmeansObj, img, imgNullVal):
    nBands, nRows, nCols = img.shape
    xFull = np.transpose(img, (1, 2, 0)).reshape((nRows * nCols, nBands))
    clustersImg = kmeansObj.predict(xFull).reshape((nRows, nCols)).astype(np.int64)
    clustersImg += 1
    if imgNullVal is not None:
        clustersImg[(img == imgNullVal).any(axis=0)] = SEGNULLVAL
    return clustersImg


def autoMaxSpectralDiff(km, maxSpectralDiff, distPcntile):
    centres = km.cluster_centers_
    n = centres.shape[0]
    iu, ju = np.triu_indices(n, k=1)
    # mismo cómputo que la referencia: sqrt en float64, almacenado en float32
    d = np.sqrt(((centres[iu] - centres[ju]) ** 2).sum(axis=1)).astype(np.float32)
    if isinstance(maxSpectralDiff, str) and maxSpectralDiff == 'auto':
        maxSpectralDiff = np.percentile(d, distPcntile)
    elif maxSpectralDiff is None:
        maxSpectralDiff = 10 * d.max()
    return maxSpectralDiff


# ---------------------------------------------------------------- paso 2

def clump(img, ignoreVal, fourConnected=True, clumpId=1):
    """Componentes conexos con IDs en orden de descubrimiento raster.

    Vía rápida: scipy.ndimage.label por valor de cluster + renumeración por
    primera aparición en barrido raster == flood fill de la referencia,
    SIEMPRE que ningún componente alcance MAX_CLUMP_SIZE (si lo alcanza, la
    referencia lo parte; caemos al flood fill exacto, lento pero fiel).
    """
    structure = (np.array([[0, 1, 0], [1, 1, 1], [0, 1, 0]]) if fourConnected
                 else np.ones((3, 3), dtype=int))
    out = np.zeros(img.shape, dtype=np.int64)
    nextLabel = 0
    for val in np.unique(img):
        if val == ignoreVal:
            continue
        lab, n = ndimage.label(img == val, structure=structure)
        out[lab > 0] = lab[lab > 0] + nextLabel
        nextLabel += n

    sizes = np.bincount(out.ravel())
    if len(sizes) > 1 and sizes[1:].max() >= MAX_CLUMP_SIZE:
        return _clumpExact(img, ignoreVal, fourConnected, clumpId)

    # renumera por primera aparición en orden raster (= orden de descubrimiento)
    flat = out.ravel()
    nonzero = flat != 0
    first_idx = np.full(nextLabel + 1, flat.size, dtype=np.int64)
    np.minimum.at(first_idx, flat[nonzero], np.flatnonzero(nonzero))
    order = np.argsort(first_idx[1:], kind='stable')  # labels 1..nextLabel
    remap = np.zeros(nextLabel + 1, dtype=np.int64)
    remap[1 + order] = np.arange(nextLabel, dtype=np.int64) + clumpId
    seg = remap[out].astype(SegIdType)
    return seg, clumpId + nextLabel


def _clumpExact(img, ignoreVal, fourConnected, clumpId):
    """Flood fill exacto de la referencia (con tope MAX_CLUMP_SIZE)."""
    ysize, xsize = img.shape
    output = np.zeros((ysize, xsize), dtype=SegIdType)
    stack = []
    for y in range(ysize):
        for x in range(xsize):
            if img[y, x] != ignoreVal and output[y, x] == 0:
                val = img[y, x]
                clumpSize = 0
                stack.clear()
                stack.append((y, x))
                output[y, x] = clumpId
                while stack and clumpSize < MAX_CLUMP_SIZE:
                    sy, sx = stack.pop()
                    for cx in range(max(sx - 1, 0), min(sx + 1, xsize - 1) + 1):
                        for cy in range(max(sy - 1, 0), min(sy + 1, ysize - 1) + 1):
                            connected = not fourConnected or (cy == sy or cx == sx)
                            if (connected and img[cy, cx] != ignoreVal and
                                    output[cy, cx] == 0 and img[cy, cx] == val):
                                output[cy, cx] = clumpId
                                clumpSize += 1
                                stack.append((cy, cx))
                clumpId += 1
    return output, clumpId


# ---------------------------------------------------------------- utilidades

def makeSegSize(seg):
    return np.bincount(seg.ravel()).astype(np.int64)


def buildSegmentSpectra(seg, img, maxSegId):
    nBands = img.shape[0]
    spectSum = np.zeros((maxSegId + 1, nBands), dtype=np.float32)
    flat = seg.ravel()
    for k in range(nBands):
        # bincount acumula en float64; la referencia acumula en float32 píxel a
        # píxel — diferencia de redondeo despreciable (validada por ARI).
        spectSum[:, k] = np.bincount(flat, weights=img[k].ravel(),
                                     minlength=maxSegId + 1).astype(np.float32)
    return spectSum


def relabelSegments(seg, segSize, minSegId):
    n = len(segSize)
    subtract = np.zeros(n, dtype=np.int64)
    if n > minSegId + 1:
        subtract[minSegId + 1:] = np.cumsum(segSize[minSegId:-1] == 0)
    seg[:] = (seg.astype(np.int64) - subtract[seg]).astype(seg.dtype)


# offsets en el MISMO orden de recorrido que la referencia (ii externo, jj interno)
_OFFSETS8 = [(di, dj) for di in (-1, 0, 1) for dj in (-1, 0, 1)]


def _neighbour_candidates(rows, cols, nRows, nCols, fourConnected):
    """Vecinos de cada píxel en el orden exacto de la referencia.

    Devuelve (idx_pixel, rr, cc) aplanados con orden: píxel k → di → dj
    (equivalente al doble for ii/jj con rangos recortados en bordes).
    """
    nPix = len(rows)
    rr = np.empty((nPix, 9), dtype=np.int64)
    cc = np.empty((nPix, 9), dtype=np.int64)
    ok = np.empty((nPix, 9), dtype=bool)
    for m, (di, dj) in enumerate(_OFFSETS8):
        rr[:, m] = rows + di
        cc[:, m] = cols + dj
        valid = (rr[:, m] >= 0) & (rr[:, m] < nRows) & (cc[:, m] >= 0) & (cc[:, m] < nCols)
        if fourConnected and di != 0 and dj != 0:
            valid &= False
        ok[:, m] = valid
    idx = np.repeat(np.arange(nPix), 9).reshape(nPix, 9)
    sel = ok.ravel()
    return idx.ravel()[sel], rr.ravel()[sel], cc.ravel()[sel]


# ------------------------------------------------ eliminación de píxeles sueltos

def eliminateSinglePixels(img, seg, segSize, minSegId, maxSegId, fourConnected):
    total = _mergeSinglePixels(img, seg, segSize, fourConnected)
    while total > 0:
        total = _mergeSinglePixels(img, seg, segSize, fourConnected)
    relabelSegments(seg, segSize, minSegId)


def _mergeSinglePixels(img, seg, segSize, fourConnected):
    nBands, nRows, nCols = img.shape
    singleMask = (segSize[seg] == 1)
    if not singleMask.any():
        return 0
    rows, cols = np.nonzero(singleMask)  # orden raster, como la referencia

    pidx, rr, cc = _neighbour_candidates(rows, cols, nRows, nCols, fourConnected)
    nbrSeg = seg[rr, cc]
    eligible = segSize[nbrSeg] > 1
    if not eligible.any():
        return 0
    pidx, rr, cc = pidx[eligible], rr[eligible], cc[eligible]

    # distancia espectral píxel-a-píxel (img float32 → como la referencia)
    d = img[:, rows[pidx], cols[pidx]].astype(np.float32) - img[:, rr, cc].astype(np.float32)
    dSqr = (d * d).sum(axis=0)

    # primer mínimo por píxel en el orden de candidatos (== desempate referencia)
    best = np.full(len(rows), -1, dtype=np.int64)
    bestD = np.full(len(rows), np.inf, dtype=np.float64)
    for k in range(len(pidx)):  # candidatos ya están en orden (k, di, dj)
        p = pidx[k]
        if dSqr[k] < bestD[p]:
            bestD[p] = dSqr[k]
            best[p] = k

    found = best >= 0
    if not found.any():
        return 0
    newSeg = seg[rr[best[found]], cc[best[found]]]
    oldSeg = seg[rows[found], cols[found]]
    seg[rows[found], cols[found]] = newSeg
    segSize[oldSeg] = 0
    np.add.at(segSize, newSeg, 1)
    return int(found.sum())


# ------------------------------------------------ eliminación de segmentos chicos

def eliminateSmallSegments(seg, img, maxSegId, minSegSize, maxSpectralDiff,
                           fourConnected, minSegId):
    nRows, nCols = seg.shape
    spectSum = buildSegmentSpectra(seg, img, maxSegId)
    segSize = makeSegSize(seg)
    if len(segSize) < maxSegId + 1:
        segSize = np.concatenate([segSize, np.zeros(maxSegId + 1 - len(segSize), np.int64)])

    # segLoc: rowcols por segmento EN ORDEN RASTER (el desempate depende del orden)
    flat = seg.ravel()
    order = np.argsort(flat, kind='stable')
    sortedSeg = flat[order]
    starts = np.searchsorted(sortedSeg, np.arange(maxSegId + 2))
    segLoc = {}
    for sid in range(minSegId, maxSegId + 1):
        sl = order[starts[sid]:starts[sid + 1]]
        if len(sl):
            segLoc[sid] = np.stack([sl // nCols, sl % nCols], axis=1)

    maxDiffSqr = float(maxSpectralDiff) ** 2
    numElim = 0
    for targetSize in range(1, minSegSize):
        count = int((segSize == targetSize).sum())
        prevCount = -1
        numPasses = 0
        while count != prevCount and numPasses < 10:
            prevCount = count
            ids = np.where(segSize == targetSize)[0]
            ids = ids[ids >= minSegId]
            merges = []
            for sid in ids:  # orden ascendente de ID, como la referencia
                nbr = _findMergeSegment(sid, segLoc, seg, segSize, spectSum,
                                        maxDiffSqr, fourConnected, nRows, nCols)
                if nbr != SEGNULLVAL:
                    merges.append((sid, nbr))
            for sid, nbr in merges:
                _doMerge(sid, nbr, seg, segSize, segLoc, spectSum)
                numElim += 1
            count = int((segSize == targetSize).sum())
            numPasses += 1

    relabelSegments(seg, segSize, minSegId)
    return numElim


def _findMergeSegment(segId, segLoc, seg, segSize, spectSum, maxDiffSqr,
                      fourConnected, nRows, nCols):
    rowcols = segLoc[segId]
    numPix = len(rowcols)
    spect = spectSum[segId] / np.float32(numPix)

    pidx, rr, cc = _neighbour_candidates(rowcols[:, 0], rowcols[:, 1],
                                         nRows, nCols, fourConnected)
    nbrSeg = seg[rr, cc]
    mySize = segSize[segId]
    elig = (nbrSeg != segId) & (nbrSeg != SEGNULLVAL) & (segSize[nbrSeg] > mySize)
    if not elig.any():
        return SEGNULLVAL
    nbrSeg = nbrSeg[elig]

    nbrSpect = spectSum[nbrSeg] / segSize[nbrSeg, None].astype(np.float32)
    diff = (spect[None, :] - nbrSpect).astype(np.float32)
    distSqr = (diff * diff).sum(axis=1)

    k = int(np.argmin(distSqr))  # primera ocurrencia del mínimo == referencia
    if distSqr[k] > maxDiffSqr:
        return SEGNULLVAL
    return int(nbrSeg[k])


def _doMerge(segId, nbrSegId, seg, segSize, segLoc, spectSum):
    rc = segLoc[segId]
    seg[rc[:, 0], rc[:, 1]] = nbrSegId
    # orden: vecino primero, luego el fusionado (igual que la referencia)
    segLoc[nbrSegId] = np.concatenate([segLoc[nbrSegId], rc])
    del segLoc[segId]
    spectSum[nbrSegId] += spectSum[segId]
    spectSum[segId] = 0
    segSize[nbrSegId] += segSize[segId]
    segSize[segId] = 0
