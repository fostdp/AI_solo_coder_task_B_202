const BellVertexShader = `
    uniform float uTime;
    uniform int uModeOrder;
    uniform float uModeAmplitude;
    uniform float uBellHeight;
    uniform float uBellRadius;
    uniform bool uShowVibration;

    varying vec3 vNormal;
    varying vec3 vWorldPosition;
    varying float vDisplacement;
    varying float vTheta;
    varying float vPhi;
    varying vec2 vUv;

    float computeModeDisplacement(float theta, float phi, int modeOrder) {
        float disp = 0.0;
        if (modeOrder == 0) {
            disp = sin(float(1) * theta) * sin(phi);
        } else if (modeOrder == 1) {
            disp = cos(float(2) * theta) * sin(2.0 * phi);
        } else if (modeOrder == 2) {
            disp = sin(float(3) * theta) * cos(phi);
        } else {
            disp = cos(float(4) * theta) * sin(3.0 * phi);
        }
        float edgeFactor = sin(phi);
        return disp * edgeFactor;
    }

    void main() {
        vUv = uv;
        vNormal = normalize(normalMatrix * normal);

        float radius = length(position.xz);
        vTheta = atan(position.z, position.x);
        vPhi = 3.14159265359 * (position.y + uBellHeight * 0.5) / uBellHeight;
        vPhi = clamp(vPhi, 0.0, 3.14159265359);

        float rawDisp = computeModeDisplacement(vTheta, vPhi, uModeOrder);
        vDisplacement = rawDisp;

        vec3 newPosition = position;

        if (uShowVibration) {
            float animated = rawDisp * sin(uTime * 2.5) * uModeAmplitude;
            vec3 radialDir = normalize(vec3(position.x, 0.0, position.z));
            if (length(radialDir) > 0.001) {
                newPosition += radialDir * animated * 0.8;
                newPosition.y += animated * 0.3 * sin(vPhi);
            }
        }

        vec4 worldPos = modelMatrix * vec4(newPosition, 1.0);
        vWorldPosition = worldPos.xyz;

        gl_Position = projectionMatrix * viewMatrix * worldPos;
    }
`;

const BellFragmentShader = `
    uniform float uTime;
    uniform int uModeOrder;
    uniform bool uShowContours;
    uniform bool uWireframe;
    uniform vec3 uBaseColor;

    varying vec3 vNormal;
    varying vec3 vWorldPosition;
    varying float vDisplacement;
    varying float vTheta;
    varying float vPhi;
    varying vec2 vUv;

    vec3 hsv2rgb(vec3 c) {
        vec4 K = vec4(1.0, 2.0 / 3.0, 1.0 / 3.0, 3.0);
        vec3 p = abs(fract(c.xxx + K.xyz) * 6.0 - K.www);
        return c.z * mix(K.xxx, clamp(p - K.xxx, 0.0, 1.0), c.y);
    }

    float drawIsoline(float value, float numLines, float lineWidth) {
        float scaled = value * numLines * 0.5 + numLines * 0.5;
        float line = abs(fract(scaled - 0.5) - 0.5);
        float width = fwidth(scaled) * lineWidth;
        return 1.0 - smoothstep(0.0, width, line);
    }

    void main() {
        vec3 viewDir = normalize(cameraPosition - vWorldPosition);
        float fresnel = pow(1.0 - max(dot(viewDir, vNormal), 0.0), 3.0);

        vec3 bronze = uBaseColor;
        vec3 highlight = vec3(1.0, 0.85, 0.2);

        float normalizedDisp = clamp(vDisplacement * 0.5 + 0.5, 0.0, 1.0);
        float hue = (1.0 - normalizedDisp) * 0.66;
        vec3 modeColor = hsv2rgb(vec3(hue, 0.9, 0.9));

        vec3 finalColor = mix(bronze, modeColor, 0.45);
        finalColor += fresnel * highlight * 0.35;

        if (uShowContours) {
            float isoline1 = drawIsoline(vDisplacement, 10.0, 0.8);
            float isoline2 = drawIsoline(vDisplacement, 20.0, 0.5);

            vec3 contourColor1 = hsv2rgb(vec3(hue, 1.0, 1.0));
            vec3 contourColor2 = vec3(1.0);

            finalColor = mix(finalColor, contourColor1, isoline1 * 0.85);
            finalColor = mix(finalColor, contourColor2, isoline2 * 0.35);

            float pulse = 0.5 + 0.5 * sin(uTime * 4.0);
            float zeroLine = drawIsoline(vDisplacement, 100.0, 1.2);
            finalColor = mix(finalColor, vec3(1.0, 1.0, 0.5), zeroLine * 0.5 * pulse);
        }

        float ambient = 0.35;
        vec3 lightDir = normalize(vec3(0.5, 0.8, 0.5));
        float diffuse = max(dot(vNormal, lightDir), 0.0) * 0.65;
        float lighting = ambient + diffuse;
        finalColor *= lighting;

        if (uWireframe) {
            finalColor = mix(finalColor, bronze * 1.5, 0.6);
            float gridX = abs(fract(vUv.x * 60.0) - 0.5);
            float gridY = abs(fract(vUv.y * 60.0) - 0.5);
            float gwx = fwidth(vUv.x * 60.0) * 0.8;
            float gwy = fwidth(vUv.y * 60.0) * 0.8;
            float grid = 1.0 - min(smoothstep(0.0, gwx, gridX), smoothstep(0.0, gwy, gridY));
            finalColor = mix(finalColor, vec3(1.0, 0.9, 0.5), grid * 0.8);
        }

        gl_FragColor = vec4(finalColor, 1.0);
    }
`;

const GrindingVertexShader = `
    uniform float uTime;
    uniform float uDepth;

    varying vec3 vNormal;
    varying vec3 vWorldPosition;
    varying float vDist;

    void main() {
        vNormal = normalize(normalMatrix * normal);
        vDist = length(position);

        vec3 newPos = position;
        float pulse = 1.0 + 0.15 * sin(uTime * 3.0);
        newPos *= pulse;

        vec4 worldPos = modelMatrix * vec4(newPos, 1.0);
        vWorldPosition = worldPos.xyz;
        gl_Position = projectionMatrix * viewMatrix * worldPos;
    }
`;

const GrindingFragmentShader = `
    uniform float uTime;
    uniform float uDepth;

    varying vec3 vNormal;
    varying vec3 vWorldPosition;
    varying float vDist;

    void main() {
        float pulse = 0.6 + 0.4 * sin(uTime * 4.0);
        vec3 coreColor = vec3(1.0, 0.2, 0.2);
        vec3 edgeColor = vec3(1.0, 0.8, 0.0);

        float fade = 1.0 - smoothstep(0.0, 1.0, vDist);
        vec3 color = mix(edgeColor, coreColor, fade);
        color *= pulse;

        float alpha = 0.75 + 0.25 * sin(uTime * 5.0);
        gl_FragColor = vec4(color, alpha);
    }
`;

const RingVertexShader = `
    uniform float uTime;

    varying float vAngle;
    varying vec3 vWorldPosition;

    void main() {
        float angle = atan(position.y, position.x);
        vAngle = angle;

        float pulse = 1.0 + 0.1 * sin(uTime * 2.0 + angle * 3.0);
        vec3 newPos = position * pulse;

        vec4 worldPos = modelMatrix * vec4(newPos, 1.0);
        vWorldPosition = worldPos.xyz;
        gl_Position = projectionMatrix * viewMatrix * worldPos;
    }
`;

const RingFragmentShader = `
    uniform float uTime;
    varying float vAngle;
    varying vec3 vWorldPosition;

    void main() {
        float scan = 0.5 + 0.5 * sin(uTime * 3.0 - vAngle * 4.0);
        vec3 color = mix(vec3(1.0, 0.0, 0.0), vec3(1.0, 0.6, 0.0), scan);
        float alpha = 0.4 + 0.3 * scan;
        gl_FragColor = vec4(color, alpha);
    }
`;

const DecorativeRingVertexShader = `
    uniform float uTime;
    uniform float uRingY;
    uniform float uRingScale;

    varying vec3 vNormal;
    varying vec3 vWorldPosition;
    varying float vRingFactor;

    void main() {
        vNormal = normalize(normalMatrix * normal);
        vRingFactor = uRingScale;

        vec3 newPos = position;
        vec4 worldPos = modelMatrix * vec4(newPos, 1.0);
        vWorldPosition = worldPos.xyz;
        gl_Position = projectionMatrix * viewMatrix * worldPos;
    }
`;

const DecorativeRingFragmentShader = `
    uniform float uTime;
    uniform vec3 uBaseColor;

    varying vec3 vNormal;
    varying vec3 vWorldPosition;
    varying float vRingFactor;

    void main() {
        vec3 viewDir = normalize(cameraPosition - vWorldPosition);
        float fresnel = pow(1.0 - max(dot(viewDir, vNormal), 0.0), 3.0);

        vec3 bronze = uBaseColor * 0.85;
        vec3 highlight = vec3(1.0, 0.9, 0.3);

        float ambient = 0.35;
        vec3 lightDir = normalize(vec3(0.5, 0.8, 0.5));
        float diffuse = max(dot(vNormal, lightDir), 0.0) * 0.7;
        vec3 color = bronze * (ambient + diffuse);
        color += fresnel * highlight * 0.5;

        float specularBase = pow(max(dot(reflect(-lightDir, vNormal), viewDir), 0.0), 48.0);
        color += vec3(1.0, 0.95, 0.6) * specularBase * 0.6;

        gl_FragColor = vec4(color, 1.0);
    }
`;

function isMobileDevice() {
    if (typeof navigator === 'undefined') return false;
    const ua = navigator.userAgent || navigator.vendor || '';
    return /android|webos|iphone|ipad|ipod|blackberry|iemobile|opera mini|mobile/i.test(ua) ||
           (typeof window !== 'undefined' && window.innerWidth < 768);
}

function getLowPowerMode() {
    if (typeof navigator !== 'undefined' && navigator.hardwareConcurrency) {
        if (navigator.hardwareConcurrency <= 4) return true;
    }
    if (typeof navigator !== 'undefined' && navigator.deviceMemory) {
        if (navigator.deviceMemory <= 4) return true;
    }
    return isMobileDevice();
}

class Bell3DViewer {
    constructor(containerId) {
        this.container = document.getElementById(containerId);
        this.scene = null;
        this.camera = null;
        this.renderer = null;
        this.bellMesh = null;
        this.bellMaterial = null;
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
        this.clock = new THREE.Clock();
        this.uniforms = null;

        this.isMobile = isMobileDevice();
        this.lowPower = getLowPowerMode();
        this.lodLevel = this.lowPower ? 2 : (this.isMobile ? 1 : 0);
        this.frameTimes = [];
        this.fps = 60;
        this.lastFpsUpdate = 0;
        this.frameCount = 0;
        this.adaptiveLodTriggered = false;

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

        let maxPixelRatio = 2;
        if (this.lodLevel >= 2) {
            maxPixelRatio = 1;
        } else if (this.lodLevel >= 1) {
            maxPixelRatio = Math.min(window.devicePixelRatio, 1.5);
        }

        this.renderer = new THREE.WebGLRenderer({
            antialias: this.lodLevel < 2,
            powerPreference: "high-performance",
            preserveDrawingBuffer: false,
            alpha: false
        });
        this.renderer.setSize(width, height);
        this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, maxPixelRatio));
        if (this.lodLevel < 1) {
            this.renderer.shadowMap.enabled = true;
        } else {
            this.renderer.shadowMap.enabled = false;
        }
        this.container.appendChild(this.renderer.domElement);

        this.setupLights();
        if (this.lodLevel < 2) {
            this.setupGrid();
        }

        window.addEventListener('resize', () => this.onResize());
        this.renderer.domElement.addEventListener('mousedown', (e) => this.onMouseDown(e));
        this.renderer.domElement.addEventListener('mousemove', (e) => this.onMouseMove(e));
        this.renderer.domElement.addEventListener('mouseup', () => this.onMouseUp());
        this.renderer.domElement.addEventListener('wheel', (e) => this.onWheel(e), { passive: false });
        if (this.isMobile) {
            this.renderer.domElement.addEventListener('touchstart', (e) => this.onTouchStart(e), { passive: false });
            this.renderer.domElement.addEventListener('touchmove', (e) => this.onTouchMove(e), { passive: false });
            this.renderer.domElement.addEventListener('touchend', () => this.onTouchEnd());
        }

        this.animate();
    }

    setupLights() {
        const ambientLight = new THREE.AmbientLight(0x404060, 0.6);
        this.scene.add(ambientLight);

        const mainLight = new THREE.DirectionalLight(0xffd700, 1.0);
        mainLight.position.set(100, 200, 100);
        mainLight.castShadow = true;
        this.scene.add(mainLight);

        const fillLight = new THREE.DirectionalLight(0x4488ff, 0.5);
        fillLight.position.set(-100, 100, -100);
        this.scene.add(fillLight);

        const rimLight = new THREE.DirectionalLight(0xff8800, 0.4);
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

    createBellShaderMaterial() {
        this.uniforms = {
            uTime: { value: 0 },
            uModeOrder: { value: 0 },
            uModeAmplitude: { value: 3.0 },
            uBellHeight: { value: 80.0 },
            uBellRadius: { value: 20.0 },
            uShowVibration: { value: true },
            uShowContours: { value: true },
            uWireframe: { value: false },
            uBaseColor: { value: new THREE.Color(0xb8860b) }
        };

        this.bellMaterial = new THREE.ShaderMaterial({
            uniforms: this.uniforms,
            vertexShader: BellVertexShader,
            fragmentShader: BellFragmentShader,
            side: THREE.DoubleSide
        });

        return this.bellMaterial;
    }

    createBellGeometry(bell) {
        this.currentBell = bell;
        const height = bell.height_cm || 80;
        const bottomRadius = (bell.diameter_cm || 40) / 2;
        const topRadius = bottomRadius * 0.6;
        const thickness = (bell.thickness_mm || 15) / 10;

        if (this.uniforms) {
            this.uniforms.uBellHeight.value = height;
            this.uniforms.uBellRadius.value = bottomRadius;
        }

        let segments, heightSegments;
        switch (this.lodLevel) {
            case 2:
                segments = 32;
                heightSegments = 16;
                break;
            case 1:
                segments = 64;
                heightSegments = 32;
                break;
            default:
                segments = 96;
                heightSegments = 48;
        }

        const outerPoints = [];
        const innerPoints = [];

        for (let i = 0; i <= heightSegments; i++) {
            const t = i / heightSegments;
            const y = -height / 2 + t * height;
            const profile = 1 - 0.15 * Math.sin(t * Math.PI);
            const outerR = (topRadius + (bottomRadius - topRadius) * t) * profile;
            const innerR = Math.max(outerR - thickness, 1);

            outerPoints.push(new THREE.Vector2(outerR, y));
            innerPoints.push(new THREE.Vector2(innerR, y));
        }

        const outerGeom = new THREE.LatheGeometry(outerPoints, segments);
        const innerGeom = new THREE.LatheGeometry(innerPoints, segments);
        innerGeom.scale(-1, 1, 1);

        const topOuterR = topRadius;
        const topInnerR = Math.max(topRadius - thickness, 1);
        const topRingSegments = this.lodLevel >= 2 ? 32 : segments;
        const topRingGeom = new THREE.RingGeometry(topInnerR, topOuterR, topRingSegments);
        topRingGeom.rotateX(-Math.PI / 2);
        topRingGeom.translate(0, height / 2, 0);

        const bottomOuterR = bottomRadius;
        const bottomInnerR = Math.max(bottomRadius - thickness, 1);
        const bottomRingGeom = new THREE.RingGeometry(bottomInnerR, bottomOuterR, topRingSegments);
        bottomRingGeom.rotateX(Math.PI / 2);
        bottomRingGeom.translate(0, -height / 2, 0);

        const material = this.createBellShaderMaterial();

        const outerMesh = new THREE.Mesh(outerGeom, material);
        const innerMesh = new THREE.Mesh(innerGeom, material);
        const topRing = new THREE.Mesh(topRingGeom, material);
        const bottomRing = new THREE.Mesh(bottomRingGeom, material);

        if (this.lodLevel < 1) {
            outerMesh.castShadow = true;
            outerMesh.receiveShadow = true;
        }

        this.bellMesh = new THREE.Group();
        this.bellMesh.add(outerMesh);
        this.bellMesh.add(innerMesh);
        this.bellMesh.add(topRing);
        this.bellMesh.add(bottomRing);

        if (this.lodLevel < 2) {
            this.addDecorativeRings(height, bottomRadius, topRadius, segments);
        }

        this.scene.add(this.bellMesh);
    }

    addDecorativeRings(height, bottomRadius, topRadius, segments) {
        let torusSegments = this.lodLevel >= 1 ? 16 : 32;
        let torusRadialSegments = this.lodLevel >= 1 ? 4 : 8;

        const ringUniforms = {
            uTime: { value: 0 },
            uRingY: { value: 0 },
            uRingScale: { value: 1.0 },
            uBaseColor: { value: new THREE.Color(0x8b6914) }
        };

        const ringMaterial = new THREE.ShaderMaterial({
            uniforms: ringUniforms,
            vertexShader: DecorativeRingVertexShader,
            fragmentShader: DecorativeRingFragmentShader
        });

        this.sharedRingUniforms = ringUniforms;

        for (let t = 0.2; t < 0.9; t += 0.2) {
            const y = -height / 2 + t * height;
            const r = (topRadius + (bottomRadius - topRadius) * t) * (1 - 0.15 * Math.sin(t * Math.PI));

            const ringGeom = new THREE.TorusGeometry(r, 0.5, torusRadialSegments, torusSegments);
            ringGeom.rotateX(Math.PI / 2);
            ringGeom.translate(0, y, 0);

            const ring = new THREE.Mesh(ringGeom, ringMaterial);
            this.bellMesh.add(ring);
        }
    }

    addGrindingMarker(position, depth = 0.5) {
        const radius = 1.5 + depth * 0.5;

        const uniforms = {
            uTime: { value: 0 },
            uDepth: { value: depth }
        };

        const markerGeom = new THREE.SphereGeometry(radius, 32, 32);
        const markerMat = new THREE.ShaderMaterial({
            uniforms: uniforms,
            vertexShader: GrindingVertexShader,
            fragmentShader: GrindingFragmentShader,
            transparent: true,
            depthWrite: false,
            blending: THREE.AdditiveBlending
        });
        const marker = new THREE.Mesh(markerGeom, markerMat);
        marker.position.set(position.x, position.y, position.z);

        const ringGeom = new THREE.RingGeometry(radius, radius * 1.8, 64);
        const ringMat = new THREE.ShaderMaterial({
            uniforms: { uTime: { value: 0 } },
            vertexShader: RingVertexShader,
            fragmentShader: RingFragmentShader,
            transparent: true,
            side: THREE.DoubleSide,
            depthWrite: false
        });
        const ring = new THREE.Mesh(ringGeom, ringMat);
        ring.position.copy(marker.position);
        ring.lookAt(new THREE.Vector3(0, position.y, 0));

        const group = new THREE.Group();
        group.add(marker);
        group.add(ring);
        group.userData = { type: 'grinding', depth: depth, position: position, uniforms: [uniforms, ringMat.uniforms] };

        this.grindingMarkers.push(group);
        this.scene.add(group);

        return group;
    }

    clearGrindingMarkers() {
        this.grindingMarkers.forEach(m => this.scene.remove(m));
        this.grindingMarkers = [];
    }

    setModeOrder(order) {
        this.modeOrder = order;
        if (this.uniforms) {
            this.uniforms.uModeOrder.value = order;
        }
    }

    toggleContours(show) {
        this.showContours = show;
        if (this.uniforms) {
            this.uniforms.uShowContours.value = show;
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
        if (this.uniforms) {
            this.uniforms.uWireframe.value = wireframe;
        }
    }

    clearBell() {
        if (this.bellMesh) {
            this.scene.remove(this.bellMesh);
            this.bellMesh = null;
        }
        this.clearGrindingMarkers();
        this.bellMaterial = null;
        this.uniforms = null;
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

    onTouchStart(e) {
        if (e.touches.length === 1) {
            e.preventDefault();
            this.isDragging = true;
            this.previousMousePosition = {
                x: e.touches[0].clientX,
                y: e.touches[0].clientY
            };
        }
    }

    onTouchMove(e) {
        if (e.touches.length === 1 && this.isDragging) {
            e.preventDefault();
            const deltaX = e.touches[0].clientX - this.previousMousePosition.x;
            const deltaY = e.touches[0].clientY - this.previousMousePosition.y;

            this.cameraAngle.theta += deltaX * 0.005;
            this.cameraAngle.phi = Math.max(0.1, Math.min(Math.PI - 0.1, this.cameraAngle.phi + deltaY * 0.005));

            this.updateCameraPosition();
            this.previousMousePosition = {
                x: e.touches[0].clientX,
                y: e.touches[0].clientY
            };
        }
    }

    onTouchEnd() {
        this.isDragging = false;
    }

    updateFps(now) {
        this.frameCount++;
        if (now - this.lastFpsUpdate >= 1000) {
            this.fps = this.frameCount * 1000 / (now - this.lastFpsUpdate);
            this.frameCount = 0;
            this.lastFpsUpdate = now;

            if (!this.adaptiveLodTriggered && this.lodLevel < 2 && this.fps < 25) {
                this.adaptiveLodTriggered = true;
                this.triggerAdaptiveDowngrade();
            }
        }
    }

    triggerAdaptiveDowngrade() {
        this.lodLevel = Math.min(2, this.lodLevel + 1);
        if (this.currentBell) {
            const bell = this.currentBell;
            this.clearBell();
            this.createBellGeometry(bell);
        }
        this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, this.lodLevel >= 2 ? 1 : 1.5));
        this.renderer.shadowMap.enabled = this.lodLevel < 1;
        if (this.lodLevel >= 2) {
            this.setupLights = function() {
                const ambientLight = new THREE.AmbientLight(0x404060, 0.6);
                this.scene.add(ambientLight);
                const mainLight = new THREE.DirectionalLight(0xffd700, 0.8);
                mainLight.position.set(100, 200, 100);
                this.scene.add(mainLight);
            }.bind(this);
        }
    }

    animate() {
        requestAnimationFrame(() => this.animate());

        const now = performance.now();
        this.updateFps(now);

        const elapsed = this.clock.getElapsedTime();

        if (this.uniforms) {
            this.uniforms.uTime.value = elapsed;
        }

        if (this.sharedRingUniforms) {
            this.sharedRingUniforms.uTime.value = elapsed;
        }

        this.grindingMarkers.forEach(group => {
            if (group.userData && group.userData.uniforms) {
                group.userData.uniforms.forEach(u => {
                    if (u.uTime) u.uTime.value = elapsed;
                });
            }
        });

        if (this.autoRotate && !this.isDragging) {
            this.cameraAngle.theta += 0.003;
            this.updateCameraPosition();
        }

        this.renderer.render(this.scene, this.camera);
    }
}
