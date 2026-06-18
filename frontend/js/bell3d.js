class Bell3DViewer {
    constructor(containerId) {
        this.container = document.getElementById(containerId);
        this.scene = null;
        this.camera = null;
        this.renderer = null;
        this.bellMesh = null;
        this.contourLines = [];
        this.grindingMarkers = [];
        this.modeOrder = 0;
        this.showContours = true;
        this.showGrinding = true;
        this.autoRotate = false;
        this.wireframe = false;
        this.raycaster = new THREE.Raycaster();
        this.mouse = new THREE.Vector2();
        this.modeShapes = null;
        this.currentBell = null;
        this.isDragging = false;
        this.previousMousePosition = { x: 0, y: 0 };
        this.cameraAngle = { theta: 0.5, phi: 0.8 };
        this.cameraDistance = 300;

        this.init();
    }

    init() {
        this.scene = new THREE.Scene();
        this.scene.background = new THREE.Color(0x050810);
        this.scene.fog = new THREE.Fog(0x050810, 400, 800);

        const width = this.container.clientWidth;
        const height = this.container.clientHeight;

        this.camera = new THREE.PerspectiveCamera(45, width / height, 0.1, 2000);
        this.updateCameraPosition();

        this.renderer = new THREE.WebGLRenderer({ antialias: true });
        this.renderer.setSize(width, height);
        this.renderer.setPixelRatio(window.devicePixelRatio);
        this.renderer.shadowMap.enabled = true;
        this.container.appendChild(this.renderer.domElement);

        this.setupLights();
        this.setupGrid();

        window.addEventListener('resize', () => this.onResize());
        this.renderer.domElement.addEventListener('mousedown', (e) => this.onMouseDown(e));
        this.renderer.domElement.addEventListener('mousemove', (e) => this.onMouseMove(e));
        this.renderer.domElement.addEventListener('mouseup', () => this.onMouseUp());
        this.renderer.domElement.addEventListener('wheel', (e) => this.onWheel(e));

        this.animate();
    }

    setupLights() {
        const ambientLight = new THREE.AmbientLight(0x404060, 0.4);
        this.scene.add(ambientLight);

        const mainLight = new THREE.DirectionalLight(0xffd700, 0.8);
        mainLight.position.set(100, 200, 100);
        mainLight.castShadow = true;
        this.scene.add(mainLight);

        const fillLight = new THREE.DirectionalLight(0x4488ff, 0.4);
        fillLight.position.set(-100, 100, -100);
        this.scene.add(fillLight);

        const rimLight = new THREE.DirectionalLight(0xff8800, 0.3);
        rimLight.position.set(0, -100, 100);
        this.scene.add(rimLight);
    }

    setupGrid() {
        const gridHelper = new THREE.GridHelper(400, 20, 0x2a3548, 0x1a2235);
        gridHelper.position.y = -100;
        this.scene.add(gridHelper);

        const axesHelper = new THREE.AxesHelper(50);
        axesHelper.position.y = -99;
        this.scene.add(axesHelper);
    }

    updateCameraPosition() {
        const { theta, phi } = this.cameraAngle;
        this.camera.position.x = this.cameraDistance * Math.sin(phi) * Math.cos(theta);
        this.camera.position.y = this.cameraDistance * Math.cos(phi);
        this.camera.position.z = this.cameraDistance * Math.sin(phi) * Math.sin(theta);
        this.camera.lookAt(0, 0, 0);
    }

    createBellGeometry(bell) {
        this.currentBell = bell;
        const height = bell.height_cm || 80;
        const bottomRadius = (bell.diameter_cm || 40) / 2;
        const topRadius = bottomRadius * 0.6;
        const thickness = (bell.thickness_mm || 15) / 10;

        const segments = 64;
        const heightSegments = 32;

        const outerPoints = [];
        const innerPoints = [];

        for (let i = 0; i <= heightSegments; i++) {
            const t = i / heightSegments;
            const y = -height / 2 + t * height;
            const profile = 1 - 0.15 * Math.sin(t * Math.PI);
            const outerR = (topRadius + (bottomRadius - topRadius) * t) * profile;
            const innerR = outerR - thickness;

            outerPoints.push(new THREE.Vector2(outerR, y));
            innerPoints.push(new THREE.Vector2(Math.max(innerR, 1), y));
        }

        const outerGeom = new THREE.LatheGeometry(outerPoints, segments);
        const innerGeom = new THREE.LatheGeometry(innerPoints, segments);
        innerGeom.scale(-1, 1, 1);

        const topOuterR = topRadius;
        const topInnerR = Math.max(topRadius - thickness, 1);
        const topRingGeom = new THREE.RingGeometry(topInnerR, topOuterR, segments);
        topRingGeom.rotateX(-Math.PI / 2);
        topRingGeom.translate(0, height / 2, 0);

        const bottomOuterR = bottomRadius;
        const bottomInnerR = Math.max(bottomRadius - thickness, 1);
        const bottomRingGeom = new THREE.RingGeometry(bottomInnerR, bottomOuterR, segments);
        bottomRingGeom.rotateX(Math.PI / 2);
        bottomRingGeom.translate(0, -height / 2, 0);

        const bronzeMaterial = new THREE.MeshPhongMaterial({
            color: 0xb8860b,
            shininess: 80,
            specular: 0xffd700,
            wireframe: this.wireframe,
            side: THREE.DoubleSide
        });

        const outerMesh = new THREE.Mesh(outerGeom, bronzeMaterial);
        const innerMesh = new THREE.Mesh(innerGeom, bronzeMaterial);
        const topRing = new THREE.Mesh(topRingGeom, bronzeMaterial);
        const bottomRing = new THREE.Mesh(bottomRingGeom, bronzeMaterial);

        outerMesh.castShadow = true;
        outerMesh.receiveShadow = true;

        this.bellMesh = new THREE.Group();
        this.bellMesh.add(outerMesh);
        this.bellMesh.add(innerMesh);
        this.bellMesh.add(topRing);
        this.bellMesh.add(bottomRing);

        this.addDecorativeRings(height, bottomRadius, topRadius, segments);

        this.scene.add(this.bellMesh);
    }

    addDecorativeRings(height, bottomRadius, topRadius, segments) {
        const ringMaterial = new THREE.MeshPhongMaterial({
            color: 0x8b6914,
            shininess: 100,
            specular: 0xffea00
        });

        for (let t = 0.2; t < 0.9; t += 0.2) {
            const y = -height / 2 + t * height;
            const r = (topRadius + (bottomRadius - topRadius) * t) * (1 - 0.15 * Math.sin(t * Math.PI));

            const ringGeom = new THREE.TorusGeometry(r, 0.5, 8, segments);
            ringGeom.rotateX(Math.PI / 2);
            ringGeom.translate(0, y, 0);

            const ring = new THREE.Mesh(ringGeom, ringMaterial);
            this.bellMesh.add(ring);
        }
    }

    generateModeShapeData(modeOrder) {
        if (!this.currentBell) return [];

        const points = [];
        const height = this.currentBell.height_cm || 80;
        const bottomRadius = (this.currentBell.diameter_cm || 40) / 2;
        const topRadius = bottomRadius * 0.6;
        const segments = 48;
        const heightSegs = 24;

        for (let i = 0; i <= heightSegs; i++) {
            const t = i / heightSegs;
            const y = -height / 2 + t * height;
            const profile = 1 - 0.15 * Math.sin(t * Math.PI);
            const r = (topRadius + (bottomRadius - topRadius) * t) * profile;

            for (let j = 0; j < segments; j++) {
                const theta = (j / segments) * Math.PI * 2;
                const phi = t * Math.PI;

                let displacement;
                switch (modeOrder % 4) {
                    case 0:
                        displacement = Math.sin((modeOrder + 1) * theta) * Math.sin(phi);
                        break;
                    case 1:
                        displacement = Math.cos((modeOrder + 1) * theta) * Math.sin(2 * phi);
                        break;
                    case 2:
                        displacement = Math.sin((modeOrder + 1) * theta) * Math.cos(phi);
                        break;
                    case 3:
                        displacement = Math.cos((modeOrder + 1) * theta) * Math.sin(3 * phi);
                        break;
                }

                const edgeFactor = Math.sin(t * Math.PI);
                displacement *= edgeFactor;

                points.push({
                    x: r * Math.cos(theta),
                    y: y,
                    z: r * Math.sin(theta),
                    displacement: displacement,
                    stress: Math.abs(displacement) * (1 + Math.random() * 0.2)
                });
            }
        }

        return points;
    }

    displacementToColor(displacement, maxDisp) {
        const normalized = (displacement + maxDisp) / (2 * maxDisp);
        const clamped = Math.max(0, Math.min(1, normalized));

        const hue = (1 - clamped) * 0.66;
        const color = new THREE.Color();
        color.setHSL(hue, 1, 0.5);
        return color;
    }

    createContourLines(modeOrder) {
        this.clearContourLines();

        const points = this.generateModeShapeData(modeOrder);
        if (points.length === 0) return;

        let maxDisp = 0;
        points.forEach(p => {
            maxDisp = Math.max(maxDisp, Math.abs(p.displacement));
        });
        if (maxDisp === 0) maxDisp = 1;

        const numContours = 12;
        const contourLevels = [];
        for (let i = 0; i < numContours; i++) {
            contourLevels.push(-maxDisp + (2 * maxDisp * i) / (numContours - 1));
        }

        const height = this.currentBell.height_cm || 80;
        const bottomRadius = (this.currentBell.diameter_cm || 40) / 2;
        const topRadius = bottomRadius * 0.6;
        const segments = 96;
        const heightSegs = 48;

        const displacementField = (theta, t) => {
            const phi = t * Math.PI;
            let displacement;
            switch (modeOrder % 4) {
                case 0: displacement = Math.sin((modeOrder + 1) * theta) * Math.sin(phi); break;
                case 1: displacement = Math.cos((modeOrder + 1) * theta) * Math.sin(2 * phi); break;
                case 2: displacement = Math.sin((modeOrder + 1) * theta) * Math.cos(phi); break;
                case 3: displacement = Math.cos((modeOrder + 1) * theta) * Math.sin(3 * phi); break;
            }
            return displacement * Math.sin(t * Math.PI);
        };

        contourLevels.forEach((level, levelIdx) => {
            const linePoints = [];

            for (let i = 0; i < heightSegs; i++) {
                for (let j = 0; j < segments; j++) {
                    const t1 = i / heightSegs;
                    const t2 = (i + 1) / heightSegs;
                    const theta1 = (j / segments) * Math.PI * 2;
                    const theta2 = ((j + 1) / segments) * Math.PI * 2;

                    const d00 = displacementField(theta1, t1);
                    const d10 = displacementField(theta2, t1);
                    const d01 = displacementField(theta1, t2);
                    const d11 = displacementField(theta2, t2);

                    const crossings = [];

                    if ((d00 - level) * (d10 - level) < 0) {
                        const alpha = (level - d00) / (d10 - d00);
                        crossings.push({ theta: theta1 + alpha * (theta2 - theta1), t: t1 });
                    }
                    if ((d01 - level) * (d11 - level) < 0) {
                        const alpha = (level - d01) / (d11 - d01);
                        crossings.push({ theta: theta1 + alpha * (theta2 - theta1), t: t2 });
                    }
                    if ((d00 - level) * (d01 - level) < 0) {
                        const alpha = (level - d00) / (d01 - d00);
                        crossings.push({ theta: theta1, t: t1 + alpha * (t2 - t1) });
                    }
                    if ((d10 - level) * (d11 - level) < 0) {
                        const alpha = (level - d10) / (d11 - d10);
                        crossings.push({ theta: theta2, t: t1 + alpha * (t2 - t1) });
                    }

                    crossings.forEach(c => {
                        const y = -height / 2 + c.t * height;
                        const profile = 1 - 0.15 * Math.sin(c.t * Math.PI);
                        const r = (topRadius + (bottomRadius - topRadius) * c.t) * profile + 0.3;

                        linePoints.push(new THREE.Vector3(
                            r * Math.cos(c.theta),
                            y,
                            r * Math.sin(c.theta)
                        ));
                    });
                }
            }

            if (linePoints.length > 1) {
                const color = this.displacementToColor(level, maxDisp);
                const geometry = new THREE.BufferGeometry().setFromPoints(linePoints);
                const material = new THREE.LineBasicMaterial({
                    color: color,
                    transparent: true,
                    opacity: 0.85,
                    linewidth: 2
                });

                const lines = new THREE.LinePoints(geometry, material);
                this.contourLines.push(lines);
                this.scene.add(lines);
            }
        });

        this.updateBellColorsWithDisplacement(modeOrder);
    }

    updateBellColorsWithDisplacement(modeOrder) {
        if (!this.bellMesh) return;

        this.bellMesh.traverse((child) => {
            if (child.isMesh && child.geometry && child.geometry.attributes && child.geometry.attributes.position) {
                const positions = child.geometry.attributes.position;
                const colors = new Float32Array(positions.count * 3);

                const height = this.currentBell.height_cm || 80;

                for (let i = 0; i < positions.count; i++) {
                    const x = positions.getX(i);
                    const y = positions.getY(i);
                    const z = positions.getZ(i);

                    const theta = Math.atan2(z, x);
                    const t = (y + height / 2) / height;
                    const phi = t * Math.PI;

                    let displacement;
                    switch (modeOrder % 4) {
                        case 0: displacement = Math.sin((modeOrder + 1) * theta) * Math.sin(phi); break;
                        case 1: displacement = Math.cos((modeOrder + 1) * theta) * Math.sin(2 * phi); break;
                        case 2: displacement = Math.sin((modeOrder + 1) * theta) * Math.cos(phi); break;
                        case 3: displacement = Math.cos((modeOrder + 1) * theta) * Math.sin(3 * phi); break;
                    }
                    displacement *= Math.sin(t * Math.PI);

                    const color = this.displacementToColor(displacement, 1);

                    if (child.material.wireframe) {
                        colors[i * 3] = 0.75;
                        colors[i * 3 + 1] = 0.53;
                        colors[i * 3 + 2] = 0.04;
                    } else {
                        const baseR = 0.72, baseG = 0.53, baseB = 0.04;
                        const blend = 0.35;
                        colors[i * 3] = baseR * (1 - blend) + color.r * blend;
                        colors[i * 3 + 1] = baseG * (1 - blend) + color.g * blend;
                        colors[i * 3 + 2] = baseB * (1 - blend) + color.b * blend;
                    }
                }

                child.geometry.setAttribute('color', new THREE.BufferAttribute(colors, 3));
                child.material.vertexColors = true;
                child.material.needsUpdate = true;
            }
        });
    }

    addGrindingMarker(position, depth = 0.5) {
        const radius = 1.5 + depth * 0.5;

        const markerGeom = new THREE.SphereGeometry(radius, 32, 32);
        const markerMat = new THREE.MeshBasicMaterial({
            color: 0xff3333,
            transparent: true,
            opacity: 0.8
        });
        const marker = new THREE.Mesh(markerGeom, markerMat);
        marker.position.set(position.x, position.y, position.z);
        marker.userData = { type: 'grinding', depth: depth, position: position };

        const ringGeom = new THREE.RingGeometry(radius, radius * 1.5, 32);
        const ringMat = new THREE.MeshBasicMaterial({
            color: 0xff0000,
            transparent: true,
            opacity: 0.6,
            side: THREE.DoubleSide
        });
        const ring = new THREE.Mesh(ringGeom, ringMat);

        const dir = new THREE.Vector3(position.x, 0, position.z).normalize();
        ring.position.copy(marker.position);
        ring.lookAt(new THREE.Vector3(0, position.y, 0));

        const group = new THREE.Group();
        group.add(marker);
        group.add(ring);
        group.userData = marker.userData;

        this.grindingMarkers.push(group);
        this.scene.add(group);

        return group;
    }

    clearGrindingMarkers() {
        this.grindingMarkers.forEach(m => this.scene.remove(m));
        this.grindingMarkers = [];
    }

    clearContourLines() {
        this.contourLines.forEach(l => this.scene.remove(l));
        this.contourLines = [];
    }

    clearBell() {
        if (this.bellMesh) {
            this.scene.remove(this.bellMesh);
            this.bellMesh = null;
        }
        this.clearContourLines();
        this.clearGrindingMarkers();
    }

    setModeOrder(order) {
        this.modeOrder = order;
        if (this.showContours && this.currentBell) {
            this.createContourLines(order);
        }
    }

    toggleContours(show) {
        this.showContours = show;
        if (show && this.currentBell) {
            this.createContourLines(this.modeOrder);
        } else {
            this.clearContourLines();
            this.resetBellColors();
        }
    }

    toggleGrinding(show) {
        this.showGrinding = show;
        this.grindingMarkers.forEach(m => {
            m.visible = show;
        });
    }

    toggleWireframe(wireframe) {
        this.wireframe = wireframe;
        if (this.bellMesh) {
            this.bellMesh.traverse((child) => {
                if (child.isMesh && child.material) {
                    child.material.wireframe = wireframe;
                    child.material.needsUpdate = true;
                }
            });
        }
    }

    resetBellColors() {
        if (!this.bellMesh) return;
        this.bellMesh.traverse((child) => {
            if (child.isMesh && child.material) {
                child.material.vertexColors = false;
                child.material.color.set(0xb8860b);
                child.material.needsUpdate = true;
            }
        });
    }

    onResize() {
        const width = this.container.clientWidth;
        const height = this.container.clientHeight;
        this.camera.aspect = width / height;
        this.camera.updateProjectionMatrix();
        this.renderer.setSize(width, height);
    }

    onMouseDown(e) {
        this.isDragging = true;
        this.previousMousePosition = { x: e.clientX, y: e.clientY };
    }

    onMouseMove(e) {
        const rect = this.renderer.domElement.getBoundingClientRect();
        this.mouse.x = ((e.clientX - rect.left) / rect.width) * 2 - 1;
        this.mouse.y = -((e.clientY - rect.top) / rect.height) * 2 + 1;

        if (this.isDragging) {
            const deltaX = e.clientX - this.previousMousePosition.x;
            const deltaY = e.clientY - this.previousMousePosition.y;

            this.cameraAngle.theta += deltaX * 0.005;
            this.cameraAngle.phi = Math.max(0.1, Math.min(Math.PI - 0.1, this.cameraAngle.phi + deltaY * 0.005));

            this.updateCameraPosition();
            this.previousMousePosition = { x: e.clientX, y: e.clientY };
        }
    }

    onMouseUp() {
        this.isDragging = false;
    }

    onWheel(e) {
        e.preventDefault();
        this.cameraDistance *= (1 + e.deltaY * 0.001);
        this.cameraDistance = Math.max(80, Math.min(800, this.cameraDistance));
        this.updateCameraPosition();
    }

    animate() {
        requestAnimationFrame(() => this.animate());

        if (this.autoRotate && !this.isDragging) {
            this.cameraAngle.theta += 0.003;
            this.updateCameraPosition();
        }

        if (this.grindingMarkers) {
            const time = Date.now() * 0.003;
            this.grindingMarkers.forEach((m, idx) => {
                if (m.children[1]) {
                    m.children[1].rotation.z = time + idx;
                    const scale = 1 + 0.15 * Math.sin(time * 2 + idx);
                    m.children[1].scale.set(scale, scale, scale);
                }
            });
        }

        this.renderer.render(this.scene, this.camera);
    }
}
