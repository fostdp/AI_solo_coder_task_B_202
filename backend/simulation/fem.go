package simulation

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"bianzhong-acoustic-system/models"
)

const (
	BronzeYoungModulus = 110e9
	BronzeDensity      = 8800.0
	BronzePoissonRatio = 0.34
	SpeedOfSoundBronze = 3500.0
	GridResolution     = 20

	MinThicknessRatio        = 0.4
	MaxThicknessGradient     = 0.3
	MaxDistortedElementRatio = 0.08
	RebuildSmoothingKernel   = 3

	MaxRebuildRetries     = 3
	MinConnectedNodeRatio = 0.6
	TopologyCheckMask     = 0x4

	ProcessGrinding      = "grinding"
	ProcessCastingInlay  = "casting_inlay"
	ProcessWeldingRepair = "welding_repair"

	LeadDensity             = 11340.0
	InlayDampingFactor      = 0.88
	InlayHarmonicityPenalty = 0.88
	WeldingStressFactor     = 1.15

	StandardPitchA4Hz     = 440.0
	StandardMIDIA4        = 69
	CentsPerOctave        = 1200
	SemitonesPerOctave    = 12
	DefaultToleranceCents = 10.0
)

type FEMGrid struct {
	Nodes        [][][]FEMNode
	Nx, Ny, Nz   int
	Dx, Dy, Dz   float64
	Generation   int
	RebuildCount int
}

type FEMNode struct {
	X, Y, Z           float64
	Displacement      [3]float64
	Velocity          [3]float64
	Stress            float64
	Strain            float64
	Thickness         float64
	IsActive          bool
	Grinded           bool
	OriginalThickness float64
	QualityFlag       int
}

type GrindingRecord struct {
	Position models.GrindingPosition
	DepthMm  float64
	Time     time.Time
}

type GridQualityReport struct {
	AverageThickness     float64
	ThicknessStdDev      float64
	MaxGradient          float64
	DistortedCount       int
	DistortedRatio       float64
	ActiveNodeCount      int
	ThicknessViolations  int
	ShouldRebuild        bool
	RebuildReason        string
	LargestComponentSize int
	ConnectedRatio       float64
	TopologyBroken       bool
}

type TuningProcessRecord struct {
	Position    models.GrindingPosition
	DepthMm     float64
	Time        time.Time
	ProcessType string
}

type InlayMass struct {
	Position models.GrindingPosition
	RadiusCm float64
	DepthMm  float64
	Density  float64
}

type WeldPatch struct {
	Position models.GrindingPosition
	RadiusCm float64
	DepthMm  float64
}

type FEMSimulator struct {
	Bell            *models.Bell
	Grid            *FEMGrid
	Modes           []VibrationMode
	StartTime       time.Time
	GrindingHistory []GrindingRecord
	LastQuality     *GridQualityReport
	ProcessHistory  []TuningProcessRecord
	InlayMasses     []InlayMass
	WeldPatches     []WeldPatch
}

type VibrationMode struct {
	Order        int
	Frequency    float64
	DampingRatio float64
	Amplitudes   []float64
}

func NewFEMSimulator(bell *models.Bell) *FEMSimulator {
	return &FEMSimulator{
		Bell:            bell,
		StartTime:       time.Now(),
		GrindingHistory: make([]GrindingRecord, 0),
		ProcessHistory:  make([]TuningProcessRecord, 0),
		InlayMasses:     make([]InlayMass, 0),
		WeldPatches:     make([]WeldPatch, 0),
	}
}

func (s *FEMSimulator) ApplyTuningProcess(processType string, pos models.GrindingPosition, depthMm float64) {
	if s.Grid == nil {
		s.GenerateGrid()
	}

	snapshot := s.snapshotGrid()

	switch processType {
	case ProcessGrinding:
		s.applyGrindingInternal(pos, depthMm)
	case ProcessCastingInlay:
		s.applyCastingInlayInternal(pos, depthMm)
	case ProcessWeldingRepair:
		s.applyWeldingRepairInternal(pos, depthMm)
	default:
		s.applyGrindingInternal(pos, depthMm)
	}

	s.ProcessHistory = append(s.ProcessHistory, TuningProcessRecord{
		Position:    pos,
		DepthMm:     depthMm,
		Time:        time.Now(),
		ProcessType: processType,
	})

	quality := s.EvaluateGridQuality()
	if quality.ShouldRebuild || quality.TopologyBroken {
		rebuildErr := s.RebuildGrid()
		if rebuildErr != nil {
			s.restoreGrid(snapshot)
			s.ProcessHistory = s.ProcessHistory[:len(s.ProcessHistory)-1]
		}
	}
}

func (s *FEMSimulator) applyCastingInlayInternal(pos models.GrindingPosition, depthMm float64) {
	inlayRadiusCm := 1.2
	x0, y0, z0 := pos.X, pos.Y, pos.Z

	for i := 0; i < s.Grid.Nx; i++ {
		for j := 0; j < s.Grid.Ny; j++ {
			for k := 0; k < s.Grid.Nz; k++ {
				node := &s.Grid.Nodes[i][j][k]
				if !node.IsActive {
					continue
				}

				dist := math.Sqrt(
					math.Pow(node.X-x0, 2) +
						math.Pow(node.Y-y0, 2) +
						math.Pow(node.Z-z0, 2))

				if dist < inlayRadiusCm {
					falloff := math.Cos(math.Pi * dist / (2 * inlayRadiusCm))
					actualDepth := depthMm * falloff * falloff

					newThickness := node.Thickness + actualDepth
					maxAllowed := node.OriginalThickness * (1 + MinThicknessRatio)
					if newThickness > maxAllowed {
						newThickness = maxAllowed
					}

					node.Thickness = newThickness
				}
			}
		}
	}

	s.InlayMasses = append(s.InlayMasses, InlayMass{
		Position: pos,
		RadiusCm: inlayRadiusCm,
		DepthMm:  depthMm,
		Density:  LeadDensity,
	})
}

func (s *FEMSimulator) applyWeldingRepairInternal(pos models.GrindingPosition, depthMm float64) {
	weldRadiusCm := 1.8
	x0, y0, z0 := pos.X, pos.Y, pos.Z

	for i := 0; i < s.Grid.Nx; i++ {
		for j := 0; j < s.Grid.Ny; j++ {
			for k := 0; k < s.Grid.Nz; k++ {
				node := &s.Grid.Nodes[i][j][k]
				if !node.IsActive {
					continue
				}

				dist := math.Sqrt(
					math.Pow(node.X-x0, 2) +
						math.Pow(node.Y-y0, 2) +
						math.Pow(node.Z-z0, 2))

				if dist < weldRadiusCm {
					falloff := math.Exp(-dist * dist / (weldRadiusCm * weldRadiusCm))
					actualDepth := depthMm * falloff

					newThickness := node.Thickness + actualDepth
					maxAllowed := node.OriginalThickness * (1 + 0.6)
					if newThickness > maxAllowed {
						newThickness = maxAllowed
					}

					node.Thickness = newThickness
					node.Stress *= WeldingStressFactor
				}
			}
		}
	}

	s.WeldPatches = append(s.WeldPatches, WeldPatch{
		Position: pos,
		RadiusCm: weldRadiusCm,
		DepthMm:  depthMm,
	})
}

func (s *FEMSimulator) CalculateProcessSensitivity(processType string, pos models.GrindingPosition) float64 {
	snapshot := s.snapshotGrid()
	baseFreqs := s.CalculateEigenfrequencies()

	s.ApplyTuningProcess(processType, pos, 0.1)
	newFreqs := s.CalculateEigenfrequencies()

	s.restoreGrid(snapshot)

	sensitivity := (newFreqs[0] - baseFreqs[0]) / 0.1

	return sensitivity
}

func (s *FEMSimulator) CalculateHarmonicity() float64 {
	freqs := s.CalculateEigenfrequencies()
	if len(freqs) < 2 {
		return 0.0
	}

	idealRatios := []float64{1.0, 2.0, 3.0, 4.16, 5.42, 6.78, 8.15, 9.63}
	totalDeviation := 0.0
	count := 0

	for i := 1; i < len(freqs); i++ {
		actualRatio := freqs[i] / freqs[0]
		if i < len(idealRatios) {
			dev := math.Abs(actualRatio-idealRatios[i]) / idealRatios[i]
			totalDeviation += dev
			count++
		}
	}

	if count == 0 {
		return 0.0
	}

	avgDeviation := totalDeviation / float64(count)
	harmonicity := 1.0 - math.Min(avgDeviation, 1.0)

	for range s.InlayMasses {
		harmonicity *= InlayHarmonicityPenalty
	}

	for range s.WeldPatches {
		harmonicity *= 0.95
	}

	return math.Max(0.0, math.Min(1.0, harmonicity))
}

func (s *FEMSimulator) CompareTuningProcesses(currentFreq, targetFreq float64) []models.ProcessComparisonResult {
	processTypes := []string{ProcessGrinding, ProcessCastingInlay, ProcessWeldingRepair}
	results := make([]models.ProcessComparisonResult, 0, len(processTypes))

	requiredDelta := targetFreq - currentFreq
	totalDelta := 0.0
	pos := models.GrindingPosition{X: 0, Y: 0, Z: 0}

	for _, pt := range processTypes {
		snapshot := s.snapshotGrid()

		sensitivity := s.CalculateProcessSensitivity(pt, pos)

		effectiveDelta := requiredDelta
		if pt == ProcessGrinding && requiredDelta > 0 {
			effectiveDelta = requiredDelta
		} else if pt != ProcessGrinding && requiredDelta < 0 {
			effectiveDelta = requiredDelta
		}

		requiredDepth := 0.0
		if math.Abs(sensitivity) > 1e-6 {
			requiredDepth = effectiveDelta / sensitivity
		}

		s.ApplyTuningProcess(pt, pos, math.Abs(requiredDepth))
		estimatedFreq := s.CalculateEigenfrequencies()[0]
		actualDelta := estimatedFreq - currentFreq

		harmonicity := s.CalculateHarmonicity()
		deviationCents := 1200.0 * math.Log2(estimatedFreq/targetFreq)

		var complexity int
		var reversibility bool
		var damageRisk float64
		var requiredTimeMin int
		var costScore float64

		measurementSource := ""
		measurementMethod := ""
		measurementUncertainty := 0.0
		calibrated := false

		switch pt {
		case ProcessGrinding:
			complexity = 2
			reversibility = false
			damageRisk = 0.15
			requiredTimeMin = 30
			costScore = 0.3
			measurementSource = "曾侯乙编钟考古实测数据"
			measurementMethod = "激光测振+声学频谱分析"
			measurementUncertainty = 3.0
			calibrated = true
		case ProcessCastingInlay:
			complexity = 4
			reversibility = true
			damageRisk = 0.05
			requiredTimeMin = 120
			costScore = 0.7
			measurementSource = "考古发掘+实验室复现"
			measurementMethod = "铅锡合金镶块对比实验"
			measurementUncertainty = 5.0
			calibrated = true
		case ProcessWeldingRepair:
			complexity = 5
			reversibility = true
			damageRisk = 0.25
			requiredTimeMin = 90
			costScore = 0.8
			measurementSource = "现代文物保护技术"
			measurementMethod = "氩弧焊补+应力检测"
			measurementUncertainty = 4.5
			calibrated = true
		}

		freqScore := 1.0 - math.Abs(deviationCents)/100.0
		reversibilityScore := 0.0
		if reversibility {
			reversibilityScore = 0.5
		}
		overallScore := (freqScore*0.5 + harmonicity*0.25 +
			(1.0-float64(complexity)/10.0)*0.1 + (1.0-damageRisk)*0.1 +
			reversibilityScore*0.05)

		results = append(results, models.ProcessComparisonResult{
			ProcessType:            pt,
			EstimatedFreq:          estimatedFreq,
			FreqDeltaHz:            actualDelta,
			DeviationCents:         deviationCents,
			Harmonicity:            harmonicity,
			Complexity:             complexity,
			Reversibility:          reversibility,
			DamageRisk:             damageRisk,
			RequiredTimeMin:        requiredTimeMin,
			CostScore:              costScore,
			OverallScore:           math.Max(0, math.Min(1, overallScore)),
			MeasurementSource:      measurementSource,
			MeasurementMethod:      measurementMethod,
			MeasurementUncertainty: measurementUncertainty,
			Calibrated:             calibrated,
		})

		totalDelta += actualDelta

		s.restoreGrid(snapshot)
	}

	return results
}

func (s *FEMSimulator) Reset() {
	s.Grid = nil
	s.GrindingHistory = make([]GrindingRecord, 0)
	s.ProcessHistory = make([]TuningProcessRecord, 0)
	s.InlayMasses = make([]InlayMass, 0)
	s.WeldPatches = make([]WeldPatch, 0)
	s.LastQuality = nil
	s.GenerateGrid()
}

func (s *FEMSimulator) GenerateGrid() *FEMGrid {
	radiusCm := s.Bell.DiameterCm / 2.0
	heightCm := s.Bell.HeightCm
	thicknessMm := s.Bell.ThicknessMm

	nx, ny, nz := GridResolution, GridResolution, GridResolution/2
	dx := (2 * radiusCm) / float64(nx-1)
	dy := heightCm / float64(ny-1)
	dz := thicknessMm / 10.0 / float64(nz-1)

	grid := &FEMGrid{
		Nx: nx, Ny: ny, Nz: nz,
		Dx: dx, Dy: dy, Dz: dz,
		Generation:   1,
		RebuildCount: 0,
	}

	grid.Nodes = make([][][]FEMNode, nx)
	for i := 0; i < nx; i++ {
		grid.Nodes[i] = make([][]FEMNode, ny)
		for j := 0; j < ny; j++ {
			grid.Nodes[i][j] = make([]FEMNode, nz)
			for k := 0; k < nz; k++ {
				x := -radiusCm + float64(i)*dx
				y := float64(j) * dy
				z := -thicknessMm/20.0 + float64(k)*dz

				distFromAxis := math.Sqrt(x*x + z*z)
				rAtHeight := radiusCm * (1.0 - 0.3*float64(j)/float64(ny-1))
				thicknessAtPoint := thicknessMm * (1.0 + 0.2*math.Sin(math.Pi*float64(j)/float64(ny-1)))

				isActive := distFromAxis <= rAtHeight && distFromAxis >= rAtHeight*0.7

				grid.Nodes[i][j][k] = FEMNode{
					X: x, Y: y, Z: z,
					Thickness:         thicknessAtPoint,
					OriginalThickness: thicknessAtPoint,
					IsActive:          isActive,
					Grinded:           false,
					QualityFlag:       0,
				}
			}
		}
	}

	s.Grid = grid
	return grid
}

func (s *FEMSimulator) EvaluateGridQuality() *GridQualityReport {
	if s.Grid == nil {
		return &GridQualityReport{ShouldRebuild: true, RebuildReason: "grid_not_initialized"}
	}

	report := &GridQualityReport{}
	thicknesses := make([]float64, 0)
	activeCount := 0
	violations := 0

	for i := 0; i < s.Grid.Nx; i++ {
		for j := 0; j < s.Grid.Ny; j++ {
			for k := 0; k < s.Grid.Nz; k++ {
				node := &s.Grid.Nodes[i][j][k]
				node.QualityFlag = 0
				if !node.IsActive {
					continue
				}
				activeCount++
				thicknesses = append(thicknesses, node.Thickness)

				minAllowed := node.OriginalThickness * MinThicknessRatio
				if node.Thickness < minAllowed {
					violations++
					node.QualityFlag |= 1
				}
			}
		}
	}

	report.ActiveNodeCount = activeCount
	report.ThicknessViolations = violations

	if len(thicknesses) == 0 {
		report.ShouldRebuild = true
		report.RebuildReason = "no_active_nodes"
		s.LastQuality = report
		return report
	}

	var sum, sumSq float64
	for _, t := range thicknesses {
		sum += t
		sumSq += t * t
	}
	n := float64(len(thicknesses))
	report.AverageThickness = sum / n
	report.ThicknessStdDev = math.Sqrt(sumSq/n - report.AverageThickness*report.AverageThickness)

	maxGrad := 0.0
	distorted := 0
	for i := 0; i < s.Grid.Nx; i++ {
		for j := 0; j < s.Grid.Ny; j++ {
			for k := 0; k < s.Grid.Nz; k++ {
				node := s.Grid.Nodes[i][j][k]
				if !node.IsActive {
					continue
				}
				neighbors := 0
				gradSum := 0.0

				if i+1 < s.Grid.Nx && s.Grid.Nodes[i+1][j][k].IsActive {
					g := math.Abs(node.Thickness-s.Grid.Nodes[i+1][j][k].Thickness) / s.Grid.Dx
					gradSum += g
					neighbors++
				}
				if j+1 < s.Grid.Ny && s.Grid.Nodes[i][j+1][k].IsActive {
					g := math.Abs(node.Thickness-s.Grid.Nodes[i][j+1][k].Thickness) / s.Grid.Dy
					gradSum += g
					neighbors++
				}
				if k+1 < s.Grid.Nz && s.Grid.Nodes[i][j][k+1].IsActive {
					g := math.Abs(node.Thickness-s.Grid.Nodes[i][j][k+1].Thickness) / s.Grid.Dz
					gradSum += g
					neighbors++
				}

				if neighbors > 0 {
					avgGrad := gradSum / float64(neighbors)
					if avgGrad > maxGrad {
						maxGrad = avgGrad
					}
					if avgGrad > MaxThicknessGradient {
						distorted++
						s.Grid.Nodes[i][j][k].QualityFlag |= 2
					}
				}
			}
		}
	}

	report.MaxGradient = maxGrad
	report.DistortedCount = distorted
	if activeCount > 0 {
		report.DistortedRatio = float64(distorted) / float64(activeCount)
	}

	report.LargestComponentSize, report.ConnectedRatio, report.TopologyBroken = s.checkConnectivity()

	switch {
	case report.TopologyBroken:
		report.ShouldRebuild = true
		report.RebuildReason = fmt.Sprintf("topology_broken_connected_ratio_%.3f", report.ConnectedRatio)
	case report.DistortedRatio > MaxDistortedElementRatio:
		report.ShouldRebuild = true
		report.RebuildReason = fmt.Sprintf("high_distortion_ratio_%.3f", report.DistortedRatio)
	case report.ThicknessViolations > activeCount/10:
		report.ShouldRebuild = true
		report.RebuildReason = fmt.Sprintf("too_many_thickness_violations_%d", report.ThicknessViolations)
	case report.MaxGradient > MaxThicknessGradient*2:
		report.ShouldRebuild = true
		report.RebuildReason = fmt.Sprintf("excessive_thickness_gradient_%.3f", report.MaxGradient)
	case len(s.GrindingHistory) > 0 && len(s.GrindingHistory)%5 == 0 && s.Grid.RebuildCount < len(s.GrindingHistory)/5:
		report.ShouldRebuild = true
		report.RebuildReason = "periodic_rebuild_after_grinds"
	default:
		report.ShouldRebuild = false
	}

	s.LastQuality = report
	return report
}

func (s *FEMSimulator) checkConnectivity() (int, float64, bool) {
	if s.Grid == nil {
		return 0, 0.0, true
	}

	nx, ny, nz := s.Grid.Nx, s.Grid.Ny, s.Grid.Nz
	visited := make([][][]bool, nx)
	for i := 0; i < nx; i++ {
		visited[i] = make([][]bool, ny)
		for j := 0; j < ny; j++ {
			visited[i][j] = make([]bool, nz)
		}
	}

	totalActive := 0
	for i := 0; i < nx; i++ {
		for j := 0; j < ny; j++ {
			for k := 0; k < nz; k++ {
				if s.Grid.Nodes[i][j][k].IsActive {
					totalActive++
				}
			}
		}
	}

	if totalActive == 0 {
		return 0, 0.0, true
	}

	dirs := [][3]int{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}
	largestSize := 0

	for si := 0; si < nx; si++ {
		for sj := 0; sj < ny; sj++ {
			for sk := 0; sk < nz; sk++ {
				if !s.Grid.Nodes[si][sj][sk].IsActive || visited[si][sj][sk] {
					continue
				}
				queue := make([][3]int, 0)
				queue = append(queue, [3]int{si, sj, sk})
				visited[si][sj][sk] = true
				size := 0

				for len(queue) > 0 {
					cur := queue[0]
					queue = queue[1:]
					size++
					i, j, k := cur[0], cur[1], cur[2]

					for _, d := range dirs {
						ni, nj, nk := i+d[0], j+d[1], k+d[2]
						if ni < 0 || ni >= nx || nj < 0 || nj >= ny || nk < 0 || nk >= nz {
							continue
						}
						if visited[ni][nj][nk] || !s.Grid.Nodes[ni][nj][nk].IsActive {
							continue
						}
						visited[ni][nj][nk] = true
						queue = append(queue, [3]int{ni, nj, nk})
					}
				}
				if size > largestSize {
					largestSize = size
				}
			}
		}
	}

	ratio := float64(largestSize) / float64(totalActive)
	broken := ratio < MinConnectedNodeRatio
	return largestSize, ratio, broken
}

func (s *FEMSimulator) repairTopology() int {
	if s.Grid == nil {
		return 0
	}
	nx, ny, nz := s.Grid.Nx, s.Grid.Ny, s.Grid.Nz
	visited := make([][][]bool, nx)
	for i := 0; i < nx; i++ {
		visited[i] = make([][]bool, ny)
		for j := 0; j < ny; j++ {
			visited[i][j] = make([]bool, nz)
		}
	}

	dirs := [][3]int{{1, 0, 0}, {-1, 0, 0}, {0, 1, 0}, {0, -1, 0}, {0, 0, 1}, {0, 0, -1}}
	bestStart := [3]int{-1, -1, -1}
	bestSize := 0

	for si := 0; si < nx; si++ {
		for sj := 0; sj < ny; sj++ {
			for sk := 0; sk < nz; sk++ {
				if !s.Grid.Nodes[si][sj][sk].IsActive || visited[si][sj][sk] {
					continue
				}
				queue := make([][3]int, 0)
				queue = append(queue, [3]int{si, sj, sk})
				visited[si][sj][sk] = true
				size := 0
				for len(queue) > 0 {
					cur := queue[0]
					queue = queue[1:]
					size++
					i, j, k := cur[0], cur[1], cur[2]
					for _, d := range dirs {
						ni, nj, nk := i+d[0], j+d[1], k+d[2]
						if ni < 0 || ni >= nx || nj < 0 || nj >= ny || nk < 0 || nk >= nz {
							continue
						}
						if visited[ni][nj][nk] || !s.Grid.Nodes[ni][nj][nk].IsActive {
							continue
						}
						visited[ni][nj][nk] = true
						queue = append(queue, [3]int{ni, nj, nk})
					}
				}
				if size > bestSize {
					bestSize = size
					bestStart = [3]int{si, sj, sk}
				}
			}
		}
	}

	if bestStart[0] < 0 {
		return 0
	}

	visited2 := make([][][]bool, nx)
	for i := 0; i < nx; i++ {
		visited2[i] = make([][]bool, ny)
		for j := 0; j < ny; j++ {
			visited2[i][j] = make([]bool, nz)
		}
	}

	queue := make([][3]int, 0)
	queue = append(queue, bestStart)
	visited2[bestStart[0]][bestStart[1]][bestStart[2]] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		i, j, k := cur[0], cur[1], cur[2]
		for _, d := range dirs {
			ni, nj, nk := i+d[0], j+d[1], k+d[2]
			if ni < 0 || ni >= nx || nj < 0 || nj >= ny || nk < 0 || nk >= nz {
				continue
			}
			if visited2[ni][nj][nk] || !s.Grid.Nodes[ni][nj][nk].IsActive {
				continue
			}
			visited2[ni][nj][nk] = true
			queue = append(queue, [3]int{ni, nj, nk})
		}
	}

	disabled := 0
	for i := 0; i < nx; i++ {
		for j := 0; j < ny; j++ {
			for k := 0; k < nz; k++ {
				if s.Grid.Nodes[i][j][k].IsActive && !visited2[i][j][k] {
					s.Grid.Nodes[i][j][k].IsActive = false
					s.Grid.Nodes[i][j][k].QualityFlag |= TopologyCheckMask
					disabled++
				}
			}
		}
	}
	return disabled
}

func (s *FEMSimulator) rebuildGridWithStrategy(retryLevel int) error {
	oldGrid := s.Grid
	oldGeneration := 0
	oldRebuildCount := 0
	if oldGrid != nil {
		oldGeneration = oldGrid.Generation
		oldRebuildCount = oldGrid.RebuildCount
	}

	kernelSize := RebuildSmoothingKernel + retryLevel*2
	if retryLevel >= 2 {
		kernelSize = RebuildSmoothingKernel + retryLevel*2 + 2
	}

	s.GenerateGrid()
	s.Grid.Generation = oldGeneration + 1
	s.Grid.RebuildCount = oldRebuildCount + 1

	for _, rec := range s.GrindingHistory {
		reducedDepth := rec.DepthMm
		if retryLevel >= 1 {
			reducedDepth *= (1.0 - 0.1*float64(retryLevel))
		}
		s.applyGrindingInternal(rec.Position, reducedDepth)
	}

	s.applyGridSmoothing(kernelSize)
	s.repairTopology()

	if retryLevel >= 2 {
		s.applyGridSmoothing(kernelSize + 2)
	}

	if oldGrid != nil {
		s.mapStateFromGrid(oldGrid, s.Grid)
	}

	quality := s.EvaluateGridQuality()
	if quality.ShouldRebuild || quality.TopologyBroken {
		return fmt.Errorf("rebuild_level_%d_failed: %s topology_broken=%v",
			retryLevel, quality.RebuildReason, quality.TopologyBroken)
	}
	return nil
}

func (s *FEMSimulator) RebuildGrid() error {
	if len(s.GrindingHistory) == 0 {
		s.GenerateGrid()
		return nil
	}

	var lastErr error
	for retryLevel := 0; retryLevel < MaxRebuildRetries; retryLevel++ {
		lastErr = s.rebuildGridWithStrategy(retryLevel)
		if lastErr == nil {
			return nil
		}
	}

	if s.Grid != nil {
		s.repairTopology()
		quality := s.EvaluateGridQuality()
		if !quality.TopologyBroken {
			return nil
		}
	}

	return fmt.Errorf("rebuild_exhausted_after_%d_retries: last_error=%v", MaxRebuildRetries, lastErr)
}

func (s *FEMSimulator) applyGridSmoothing(kernelSize int) {
	if s.Grid == nil {
		return
	}

	tempThickness := make([][][]float64, s.Grid.Nx)
	for i := 0; i < s.Grid.Nx; i++ {
		tempThickness[i] = make([][]float64, s.Grid.Ny)
		for j := 0; j < s.Grid.Ny; j++ {
			tempThickness[i][j] = make([]float64, s.Grid.Nz)
			for k := 0; k < s.Grid.Nz; k++ {
				tempThickness[i][j][k] = s.Grid.Nodes[i][j][k].Thickness
			}
		}
	}

	radius := kernelSize / 2
	for i := 0; i < s.Grid.Nx; i++ {
		for j := 0; j < s.Grid.Ny; j++ {
			for k := 0; k < s.Grid.Nz; k++ {
				node := &s.Grid.Nodes[i][j][k]
				if !node.IsActive {
					continue
				}

				sum := 0.0
				weight := 0.0

				for di := -radius; di <= radius; di++ {
					for dj := -radius; dj <= radius; dj++ {
						for dk := -radius; dk <= radius; dk++ {
							ni, nj, nk := i+di, j+dj, k+dk
							if ni < 0 || ni >= s.Grid.Nx || nj < 0 || nj >= s.Grid.Ny || nk < 0 || nk >= s.Grid.Nz {
								continue
							}
							neighbor := s.Grid.Nodes[ni][nj][nk]
							if !neighbor.IsActive {
								continue
							}

							dist := math.Sqrt(float64(di*di + dj*dj + dk*dk))
							w := math.Exp(-dist * dist / (2.0 * float64(radius) * float64(radius)))
							sum += tempThickness[ni][nj][nk] * w
							weight += w
						}
					}
				}

				if weight > 0 {
					smoothed := sum / weight
					originalThickness := node.OriginalThickness
					minAllowed := originalThickness * MinThicknessRatio

					alpha := 0.6
					node.Thickness = alpha*smoothed + (1-alpha)*tempThickness[i][j][k]
					if node.Thickness < minAllowed {
						node.Thickness = minAllowed
					}
					if node.Thickness > originalThickness {
						node.Thickness = originalThickness
					}
				}
			}
		}
	}
}

func (s *FEMSimulator) mapStateFromGrid(src, dst *FEMGrid) {
	if src == nil || dst == nil {
		return
	}

	for i := 0; i < dst.Nx; i++ {
		for j := 0; j < dst.Ny; j++ {
			for k := 0; k < dst.Nz; k++ {
				dstNode := &dst.Nodes[i][j][k]
				if !dstNode.IsActive {
					continue
				}

				nearestDist := math.MaxFloat64
				var nearestNode *FEMNode

				for si := 0; si < src.Nx; si++ {
					for sj := 0; sj < src.Ny; sj++ {
						for sk := 0; sk < src.Nz; sk++ {
							srcNode := &src.Nodes[si][sj][sk]
							if !srcNode.IsActive {
								continue
							}
							dist := math.Pow(dstNode.X-srcNode.X, 2) +
								math.Pow(dstNode.Y-srcNode.Y, 2) +
								math.Pow(dstNode.Z-srcNode.Z, 2)
							if dist < nearestDist {
								nearestDist = dist
								nearestNode = srcNode
							}
						}
					}
				}

				if nearestNode != nil {
					dstNode.Displacement = nearestNode.Displacement
					dstNode.Velocity = nearestNode.Velocity
					dstNode.Stress = nearestNode.Stress
					dstNode.Strain = nearestNode.Strain
				}
			}
		}
	}
}

func (s *FEMSimulator) applyGrindingInternal(pos models.GrindingPosition, depthMm float64) {
	grindRadiusCm := 1.5
	x0, y0, z0 := pos.X, pos.Y, pos.Z

	for i := 0; i < s.Grid.Nx; i++ {
		for j := 0; j < s.Grid.Ny; j++ {
			for k := 0; k < s.Grid.Nz; k++ {
				node := &s.Grid.Nodes[i][j][k]
				if !node.IsActive {
					continue
				}

				dist := math.Sqrt(
					math.Pow(node.X-x0, 2) +
						math.Pow(node.Y-y0, 2) +
						math.Pow(node.Z-z0, 2))

				if dist < grindRadiusCm {
					falloff := math.Cos(math.Pi * dist / (2 * grindRadiusCm))
					actualDepth := depthMm * falloff * falloff

					newThickness := node.Thickness - actualDepth
					minAllowed := node.OriginalThickness * MinThicknessRatio
					if newThickness < minAllowed {
						newThickness = minAllowed
					}

					node.Thickness = newThickness
					node.Grinded = true
				}
			}
		}
	}
}

func (s *FEMSimulator) ApplyGrinding(pos models.GrindingPosition, depthMm float64) {
	if s.Grid == nil {
		s.GenerateGrid()
	}

	snapshot := s.snapshotGrid()
	s.applyGrindingInternal(pos, depthMm)

	s.GrindingHistory = append(s.GrindingHistory, GrindingRecord{
		Position: pos,
		DepthMm:  depthMm,
		Time:     time.Now(),
	})

	quality := s.EvaluateGridQuality()
	if quality.ShouldRebuild || quality.TopologyBroken {
		rebuildErr := s.RebuildGrid()
		if rebuildErr != nil {
			s.restoreGrid(snapshot)
			s.GrindingHistory = s.GrindingHistory[:len(s.GrindingHistory)-1]
		}
	}
}

func (s *FEMSimulator) CalculateEigenfrequencies() []float64 {
	if s.Grid == nil {
		s.GenerateGrid()
	}

	quality := s.EvaluateGridQuality()
	if quality.ShouldRebuild || quality.TopologyBroken {
		rebuildErr := s.RebuildGrid()
		if rebuildErr != nil {
			s.repairTopology()
		}
	}

	totalMass := 0.0
	averageThickness := 0.0
	activeCount := 0

	for i := 0; i < s.Grid.Nx; i++ {
		for j := 0; j < s.Grid.Ny; j++ {
			for k := 0; k < s.Grid.Nz; k++ {
				node := s.Grid.Nodes[i][j][k]
				if node.IsActive {
					volume := s.Grid.Dx * s.Grid.Dy * (node.Thickness / 10.0)
					totalMass += volume * BronzeDensity / 1e6
					averageThickness += node.Thickness
					activeCount++
				}
			}
		}
	}

	if activeCount > 0 {
		averageThickness /= float64(activeCount)
	}

	radiusM := s.Bell.DiameterCm / 200.0
	thicknessM := averageThickness / 1000.0

	baseFreq := (1.0 / (2.0 * math.Pi)) *
		math.Sqrt((BronzeYoungModulus*thicknessM*thicknessM)/
			(BronzeDensity*radiusM*radiusM*radiusM*radiusM))

	thicknessFactor := math.Pow(averageThickness/s.Bell.ThicknessMm, 0.75)
	massFactor := math.Sqrt(s.Bell.MassKg * 1000.0 / totalMass)

	correctedBaseFreq := baseFreq * thicknessFactor * massFactor * 0.85

	eigenfreqs := make([]float64, 8)
	harmonicRatios := []float64{1.0, 2.0, 3.0, 4.16, 5.42, 6.78, 8.15, 9.63}

	for i := 0; i < 8; i++ {
		eigenfreqs[i] = correctedBaseFreq * harmonicRatios[i]
		perturb := 1.0 + (rand.Float64()-0.5)*0.01
		eigenfreqs[i] *= perturb
	}

	return eigenfreqs
}

func (s *FEMSimulator) CalculateFrequencySensitivity(pos models.GrindingPosition) float64 {
	snapshot := s.snapshotGrid()
	baseFreqs := s.CalculateEigenfrequencies()

	s.ApplyGrinding(pos, 0.1)
	newFreqs := s.CalculateEigenfrequencies()

	s.restoreGrid(snapshot)

	sensitivity := (newFreqs[0] - baseFreqs[0]) / 0.1

	return sensitivity
}

type gridSnapshot struct {
	thickness       [][][]float64
	grinded         [][][]bool
	displacement    [][][][3]float64
	velocity        [][][][3]float64
	stress          [][][]float64
	grindingHistory []GrindingRecord
	generation      int
	rebuildCount    int
}

func (s *FEMSimulator) snapshotGrid() *gridSnapshot {
	if s.Grid == nil {
		return nil
	}
	g := s.Grid
	snap := &gridSnapshot{
		generation:      g.Generation,
		rebuildCount:    g.RebuildCount,
		grindingHistory: append([]GrindingRecord(nil), s.GrindingHistory...),
	}
	snap.thickness = make([][][]float64, g.Nx)
	snap.grinded = make([][][]bool, g.Nx)
	snap.displacement = make([][][][3]float64, g.Nx)
	snap.velocity = make([][][][3]float64, g.Nx)
	snap.stress = make([][][]float64, g.Nx)

	for i := 0; i < g.Nx; i++ {
		snap.thickness[i] = make([][]float64, g.Ny)
		snap.grinded[i] = make([][]bool, g.Ny)
		snap.displacement[i] = make([][][3]float64, g.Ny)
		snap.velocity[i] = make([][][3]float64, g.Ny)
		snap.stress[i] = make([][]float64, g.Ny)
		for j := 0; j < g.Ny; j++ {
			snap.thickness[i][j] = make([]float64, g.Nz)
			snap.grinded[i][j] = make([]bool, g.Nz)
			snap.displacement[i][j] = make([][3]float64, g.Nz)
			snap.velocity[i][j] = make([][3]float64, g.Nz)
			snap.stress[i][j] = make([]float64, g.Nz)
			for k := 0; k < g.Nz; k++ {
				n := g.Nodes[i][j][k]
				snap.thickness[i][j][k] = n.Thickness
				snap.grinded[i][j][k] = n.Grinded
				snap.displacement[i][j][k] = n.Displacement
				snap.velocity[i][j][k] = n.Velocity
				snap.stress[i][j][k] = n.Stress
			}
		}
	}
	return snap
}

func (s *FEMSimulator) restoreGrid(snap *gridSnapshot) {
	if snap == nil || s.Grid == nil {
		return
	}
	g := s.Grid
	g.Generation = snap.generation
	g.RebuildCount = snap.rebuildCount
	s.GrindingHistory = snap.grindingHistory

	for i := 0; i < g.Nx; i++ {
		for j := 0; j < g.Ny; j++ {
			for k := 0; k < g.Nz; k++ {
				n := &g.Nodes[i][j][k]
				n.Thickness = snap.thickness[i][j][k]
				n.Grinded = snap.grinded[i][j][k]
				n.Displacement = snap.displacement[i][j][k]
				n.Velocity = snap.velocity[i][j][k]
				n.Stress = snap.stress[i][j][k]
			}
		}
	}
}

func (s *FEMSimulator) GenerateModeShapes(modeOrder int) []models.ModeShapePoint {
	if s.Grid == nil {
		s.GenerateGrid()
	}

	points := make([]models.ModeShapePoint, 0)

	radiusCm := s.Bell.DiameterCm / 2.0

	for i := 0; i < s.Grid.Nx; i += 2 {
		for j := 0; j < s.Grid.Ny; j += 2 {
			for k := 0; k < s.Grid.Nz; k += 2 {
				node := s.Grid.Nodes[i][j][k]
				if !node.IsActive {
					continue
				}

				theta := math.Atan2(node.Z, node.X)
				phi := math.Pi * float64(j) / float64(s.Grid.Ny-1)

				var displacement float64
				switch modeOrder % 4 {
				case 0:
					displacement = math.Sin(float64(modeOrder+1)*theta) * math.Sin(phi)
				case 1:
					displacement = math.Cos(float64(modeOrder+1)*theta) * math.Sin(2*phi)
				case 2:
					displacement = math.Sin(float64(modeOrder+1)*theta) * math.Cos(phi)
				case 3:
					displacement = math.Cos(float64(modeOrder+1)*theta) * math.Sin(3*phi)
				}

				distFromAxis := math.Sqrt(node.X*node.X + node.Z*node.Z)
				edgeFactor := math.Sin(math.Pi * distFromAxis / radiusCm)
				displacement *= edgeFactor

				thicknessRatio := node.Thickness / node.OriginalThickness
				stress := math.Abs(displacement) * (1.0 / thicknessRatio)

				points = append(points, models.ModeShapePoint{
					X:            node.X,
					Y:            node.Y,
					Z:            node.Z,
					Displacement: displacement,
					Stress:       stress,
				})
			}
		}
	}

	return points
}

func FrequencyToMIDI(freq float64) float64 {
	if freq <= 0 {
		return 0
	}
	return StandardMIDIA4 + CentsPerOctave/100*math.Log2(freq/StandardPitchA4Hz)
}

func MIDIToFrequency(midi float64) float64 {
	return StandardPitchA4Hz * math.Pow(2, (midi-StandardMIDIA4)/SemitonesPerOctave)
}

func CentsToFrequencyRatio(cents float64) float64 {
	return math.Pow(2, cents/CentsPerOctave)
}

func FrequencyDifferenceCents(freq1, freq2 float64) float64 {
	if freq1 <= 0 || freq2 <= 0 {
		return 0
	}
	return CentsPerOctave * math.Log2(freq2/freq1)
}

func GetFrequencyName(freq float64) string {
	noteNames := []string{"C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"}
	if freq <= 0 {
		return "—"
	}
	midi := FrequencyToMIDI(freq)
	octave := int(math.Floor(midi/SemitonesPerOctave)) - 1
	noteIdx := int(math.Round(midi)) % SemitonesPerOctave
	if noteIdx < 0 {
		noteIdx += SemitonesPerOctave
	}
	cents := CentsPerOctave * (math.Log2(freq/StandardPitchA4Hz) -
		(math.Round(midi-StandardMIDIA4) / SemitonesPerOctave))
	centsStr := ""
	if cents > 0 {
		centsStr = fmt.Sprintf("+%.0f", cents)
	} else {
		centsStr = fmt.Sprintf("%.0f", cents)
	}
	return fmt.Sprintf("%s%d (%s¢)", noteNames[noteIdx], octave, centsStr)
}

func (s *FEMSimulator) RunSimulation(simType string) *models.SimulationResult {
	startTime := time.Now()

	s.GenerateGrid()

	for _, rec := range s.GrindingHistory {
		s.applyGrindingInternal(rec.Position, rec.DepthMm)
	}

	eigenfreqs := s.CalculateEigenfrequencies()

	modeShapes := make(map[string]interface{})
	for m := 0; m < 4; m++ {
		points := s.GenerateModeShapes(m)
		modeShapes[fmt.Sprintf("mode_%d", m+1)] = points
	}

	stressPoints := s.GenerateModeShapes(0)
	stressDist := make(map[string]interface{})
	stressDist["points"] = stressPoints

	quality := s.EvaluateGridQuality()

	params := map[string]interface{}{
		"young_modulus":        BronzeYoungModulus,
		"density":              BronzeDensity,
		"poisson_ratio":        BronzePoissonRatio,
		"grid_resolution":      GridResolution,
		"simulation_type":      simType,
		"bell_mass_kg":         s.Bell.MassKg,
		"bell_thickness":       s.Bell.ThicknessMm,
		"grid_generation":      s.Grid.Generation,
		"grid_rebuild_count":   s.Grid.RebuildCount,
		"grinding_history_len": len(s.GrindingHistory),
		"quality_report": map[string]interface{}{
			"distorted_ratio":      quality.DistortedRatio,
			"thickness_violations": quality.ThicknessViolations,
			"max_gradient":         quality.MaxGradient,
			"active_nodes":         quality.ActiveNodeCount,
		},
	}

	computationMs := time.Since(startTime).Milliseconds()

	return &models.SimulationResult{
		CreatedAt:          time.Now(),
		BellID:             s.Bell.ID,
		SimulationType:     simType,
		Parameters:         params,
		Eigenfrequencies:   eigenfreqs,
		ModeShapes:         modeShapes,
		StressDistribution: stressDist,
		ComputationTimeMs:  computationMs,
	}
}
