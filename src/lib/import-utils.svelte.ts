import { settings } from '$lib/settings.svelte';
import jsQR from 'jsqr';
import { tick } from 'svelte';

export function maskApiKey(key: string) {
	if (!key) return '';
	if (key.length <= 8) return '****';
	return key.substring(0, 4) + '...' + key.substring(key.length - 4);
}

/**
 * Creates a camera QR scanner that can be used in any component.
 * Returns lifecycle methods: startCamera, stopCamera.
 */
export function createCameraScanner(
	getVideoElement: () => HTMLVideoElement | null,
	onSuccess: () => void
) {
	let stream: MediaStream | null = null;
	let animationFrameId: number;
	let showCameraScanner = $state(false);

	function getShowCameraScanner() {
		return showCameraScanner;
	}

	async function startCamera() {
		showCameraScanner = true;
		await tick();
		try {
			const videoEl = getVideoElement();
			
			// Try to get environment camera first, then fall back to any camera
			try {
				stream = await navigator.mediaDevices.getUserMedia({ video: { facingMode: "environment" } });
			} catch (e) {
				console.warn("Environment camera failed, falling back to default:", e);
				stream = await navigator.mediaDevices.getUserMedia({ video: true });
			}

			if (videoEl) {
				// Set listener BEFORE setting srcObject
				videoEl.onloadedmetadata = () => {
					videoEl.play().catch(e => {
						console.error("Error playing video:", e);
					});
				};
				
				videoEl.srcObject = stream;
				videoEl.setAttribute("playsinline", "true");
				requestAnimationFrame(scanLoop);
			}
		} catch (err) {
			console.error("Error accessing camera:", err);
			alert("Could not access the camera. Please check permissions.");
			showCameraScanner = false;
		}
	}

	function stopCamera() {
		if (stream) {
			stream.getTracks().forEach(track => track.stop());
			stream = null;
		}
		if (animationFrameId) {
			cancelAnimationFrame(animationFrameId);
		}
		showCameraScanner = false;
	}

	let canvas: HTMLCanvasElement | null = null;

	function scanLoop() {
		const videoEl = getVideoElement();
		if (!videoEl || videoEl.readyState !== videoEl.HAVE_ENOUGH_DATA || videoEl.videoWidth === 0) {
			if (showCameraScanner) animationFrameId = requestAnimationFrame(scanLoop);
			return;
		}

		if (!canvas) {
			canvas = document.createElement('canvas');
		}
		
		if (canvas.width !== videoEl.videoWidth || canvas.height !== videoEl.videoHeight) {
			canvas.width = videoEl.videoWidth;
			canvas.height = videoEl.videoHeight;
		}

		const ctx = canvas.getContext('2d', { willReadFrequently: true });
		if (!ctx) return;

		ctx.drawImage(videoEl, 0, 0, canvas.width, canvas.height);
		const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height);

		const code = jsQR(imageData.data, imageData.width, imageData.height, {
			inversionAttempts: "dontInvert",
		});

		if (code) {
			console.log("QR Code detected");
			if (settings.importFromQRCodeData(code.data)) {
				stopCamera();
				onSuccess();
				return;
			}
		}

		if (showCameraScanner) {
			animationFrameId = requestAnimationFrame(scanLoop);
		}
	}

	return {
		startCamera,
		stopCamera,
		getShowCameraScanner
	};
}

/**
 * Handles JSON file import for settings.
 * Returns true if import was successful.
 */
export function handleSettingsFileImport(file: File): boolean {
	const reader = new FileReader();
	let result = false;
	reader.onload = (e) => {
		const content = e.target?.result as string;
		if (settings.importSettings(content)) {
			result = true;
		} else {
			alert('Failed to import settings. Please check the file format.');
		}
	};
	reader.readAsText(file);
	return result;
}

/**
 * Handles QR image file import for settings.
 * Returns a promise that resolves to true if import was successful.
 */
export function handleQRImageImport(file: File): Promise<boolean> {
	return new Promise((resolve) => {
		const img = new Image();
		img.onload = () => {
			const canvas = document.createElement('canvas');
			const ctx = canvas.getContext('2d');
			if (!ctx) {
				resolve(false);
				return;
			}

			canvas.width = img.width;
			canvas.height = img.height;
			ctx.drawImage(img, 0, 0);

			const imageData = ctx.getImageData(0, 0, canvas.width, canvas.height);
			const code = jsQR(imageData.data, imageData.width, imageData.height);

			if (code) {
				if (settings.importFromQRCodeData(code.data)) {
					resolve(true);
				} else {
					alert('Failed to decode settings from this image.');
					resolve(false);
				}
			} else {
				alert('No QR code found in the image.');
				resolve(false);
			}
		};
		img.src = URL.createObjectURL(file);
	});
}