package simulation

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"bianzhong-acoustic-system/models"
)

const (
	BronzeYoungModulus   = 110e9
	BronzeDensity        = 8800.0
	BronzePoissonRatio   = 0.34
	SpeedOfSoundBronze   = 3500.0
	GridResolution       = 20
)

type FEMGrid struct {
	Nodes     [][][]FEMNode
	Nx, Ny, Nz int
	Dx, Dy, Dz float64
}

type FEMNode struct {
	X, Y, Z        float64
	Displacement   [3]float64
	Velocity       [3]float64
	Stress         float64
	Strain         float64
	Thickness      float64
	IsActive       bool
	Grinded        bool
	OriginalThickness float64
}

type FEMSimulator struct {
	Bell       *models.Bell
	Grid       *FEMGrid
	Modes      []VibrationMode
	StartTime  time.Time
}

type VibrationMode struct {
	Order        int
	Frequency    float64
	DampingRatio float64
	Amplitudes   []float64
}

func NewFEMSimulator(bell *models.Bell) *FEMSimulator {
	return &FEMSimulator{
		Bell:      bell,
		StartTime: time.Now(),
	}
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
				}
			}
		}
	}

	s.Grid = grid
	return grid
}

func (s *FEMSimulator) ApplyGrinding(pos models.GrindingPosition, depthMm float64) {
	if s.Grid == nil {
		s.GenerateGrid()
	}

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
					if newThickness < node.OriginalThickness*0.4 {
						newThickness = node.OriginalThickness * 0.4
					}

					node.Thickness = newThickness
					node.Grinded = true
				}
			}
		}
	}
}

func (s *FEMSimulator) CalculateEigenfrequencies() []float64 {
	if s.Grid == nil {
		s.GenerateGrid()
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
	heightM := s.Bell.HeightCm / 100.0
	thicknessM := averageThickness / 1000.0

	baseFreq := (1.0 / (2.0 * math.Pi)) *
		math.Sqrt((BronzeYoungModulus * thicknessM * thicknessM) /
			(BronzeDensity * radiusM * radiusM * radiusM * radiusM)) *
		math.Sqrt(s.Bell.TargetFrequency / baseFreq)

	thicknessFactor := math.Pow(averageThickness/s.Bell.ThicknessMm, 0.75)
	massFactor := math.Sqrt(s.Bell.MassKg*1000.0 / totalMass)

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
	baseFreqs := s.CalculateEigenfrequencies()

	s.ApplyGrinding(pos, 0.1)
	newFreqs := s.CalculateEigenfrequencies()

	sensitivity := (newFreqs[0] - baseFreqs[0]) / 0.1

	return sensitivity
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

func (s *FEMSimulator) RunSimulation(simType string) *models.SimulationResult {
	startTime := time.Now()

	s.GenerateGrid()
	eigenfreqs := s.CalculateEigenfrequencies()

	modeShapes := make(map[string]interface{})
	for m := 0; m < 4; m++ {
		points := s.GenerateModeShapes(m)
		modeShapes[fmt.Sprintf("mode_%d", m+1)] = points
	}

	stressPoints := s.GenerateModeShapes(0)
	stressDist := make(map[string]interface{})
	stressDist["points"] = stressPoints

	params := map[string]interface{}{
		"young_modulus":    BronzeYoungModulus,
		"density":          BronzeDensity,
		"poisson_ratio":    BronzePoissonRatio,
		"grid_resolution":  GridResolution,
		"simulation_type":  simType,
		"bell_mass_kg":     s.Bell.MassKg,
		"bell_thickness":   s.Bell.ThicknessMm,
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
