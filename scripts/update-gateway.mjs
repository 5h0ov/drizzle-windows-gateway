import fs from "node:fs";
import path from "node:path";
import { execSync } from "node:child_process";

const rootDir = path.resolve(import.meta.dir, "..");
const assetsDir = path.join(rootDir, "assets");
const tempDir = path.join(rootDir, ".temp_update");

console.log("=== Drizzle Gateway Native Windows Automated Builder ===");

async function getLatestVersion() {
  try {
    const res = await fetch("https://gateway.drizzle.team/docs/binary");
    const text = await res.text();
    const match = text.match(/drizzle-gateway-(\d+\.\d+\.\d+)-linux-x64/);
    if (match) return match[1];
  } catch (e) {
    console.warn("Could not reach docs for version check:", e.message);
  }
  return "1.6.0";
}

let targetVersion = process.argv[2];
if (!targetVersion || targetVersion === "latest") {
  targetVersion = await getLatestVersion();
  console.log(`Auto-detected latest Drizzle Gateway version: ${targetVersion}`);
} else {
  console.log(`Using requested Drizzle Gateway version: ${targetVersion}`);
}

const binaryPath = path.join(tempDir, `gateway-${targetVersion}`);
if (!fs.existsSync(tempDir)) fs.mkdirSync(tempDir, { recursive: true });
if (!fs.existsSync(assetsDir)) fs.mkdirSync(assetsDir, { recursive: true });

if (!fs.existsSync(binaryPath)) {
  const urls = [
    `https://pub-e240a4fd7085425baf4a7951e7611520.r2.dev/drizzle-gateway-${targetVersion}-linux-x64`,
    `https://gateway.drizzle.team/binaries/drizzle-gateway-${targetVersion}-linux-x64`
  ];
  let downloaded = false;
  for (const url of urls) {
    console.log(`Attempting download from ${url}...`);
    try {
      execSync(`curl.exe -fLo "${binaryPath}" "${url}"`, { stdio: "inherit" });
      downloaded = true;
      break;
    } catch {
      console.log(`Failed downloading from ${url}, trying next source...`);
    }
  }
  if (!downloaded) {
    throw new Error(`Failed to download Drizzle Gateway binary for version ${targetVersion}`);
  }
} else {
  console.log(`Using cached ${binaryPath}`);
}

console.log("Extracting Bun bundle...");
const buf = fs.readFileSync(binaryPath);

// Locate the Bun bundle start (the last '// @bun' marker)
const bunMarker = Buffer.from("// @bun");
let lastBunIdx = -1;
let pos = 0;
while (true) {
  const idx = buf.indexOf(bunMarker, pos);
  if (idx === -1) break;
  lastBunIdx = idx;
  pos = idx + bunMarker.length;
}

if (lastBunIdx === -1) {
  throw new Error("Could not find Bun payload marker in binary!");
}

// Find all embedded web assets matching \0/$bunfs/root/<filename>\0
const bunfsMarker = Buffer.from("/$bunfs/root/");
const webAssets = [];
pos = 0;

while (true) {
  const idx = buf.indexOf(bunfsMarker, pos);
  if (idx === -1) break;
  const nameStart = idx + bunfsMarker.length;
  const nameEnd = buf.indexOf(0, nameStart);
  const fileName = buf.subarray(nameStart, nameEnd).toString("utf8").trim();
  const contentStart = nameEnd + 1;
  
  if (/\.(svg|html|js|css)$/i.test(fileName)) {
    webAssets.push({ name: fileName, start: contentStart, markerIdx: idx });
  }
  pos = contentStart;
}

console.log(`Found ${webAssets.length} embedded web assets:`);
webAssets.forEach((f) => console.log(`  - ${f.name}`));

// Extract server.js
const firstAssetMarker = webAssets[0]?.markerIdx ?? buf.length;
let serverCode = buf.subarray(lastBunIdx, firstAssetMarker).toString("utf8");

// Trim trailing null bytes from server.js
while (serverCode.length > 0 && (serverCode.charCodeAt(serverCode.length - 1) === 0 || (serverCode.endsWith("\n") && serverCode.charCodeAt(serverCode.length - 2) === 0))) {
  serverCode = serverCode.replace(/\0+$/, "");
}

// Extract and write each web asset with exact semantic boundary trimming
const bunMagic = Buffer.from("\n---- Bun! ----\n");
const magicIdx = buf.lastIndexOf(bunMagic);

for (let i = 0; i < webAssets.length; i++) {
  const file = webAssets[i];
  const nextMarker = webAssets[i + 1]?.markerIdx ?? (magicIdx !== -1 ? magicIdx : buf.length);
  
  let fileBuf = buf.subarray(file.start, nextMarker);

  if (file.name.endsWith(".css")) {
    const lastBrace = fileBuf.lastIndexOf(125); // '}'
    if (lastBrace !== -1) fileBuf = fileBuf.subarray(0, lastBrace + 1);
  } else if (file.name.endsWith(".html")) {
    const endTag = Buffer.from("</html>");
    const endIdx = fileBuf.lastIndexOf(endTag);
    if (endIdx !== -1) fileBuf = fileBuf.subarray(0, endIdx + endTag.length);
  } else if (file.name.endsWith(".svg")) {
    const endTag = Buffer.from("</svg>");
    const endIdx = fileBuf.lastIndexOf(endTag);
    if (endIdx !== -1) fileBuf = fileBuf.subarray(0, endIdx + endTag.length);
  } else if (file.name.endsWith(".js")) {
    const endTag = Buffer.from(");");
    const endIdx = fileBuf.lastIndexOf(endTag);
    if (endIdx !== -1) fileBuf = fileBuf.subarray(0, endIdx + endTag.length);
  }

  while (fileBuf.length > 0 && fileBuf[fileBuf.length - 1] === 0) {
    fileBuf = fileBuf.subarray(0, fileBuf.length - 1);
  }

  const dest = path.join(assetsDir, file.name);
  fs.writeFileSync(dest, fileBuf);
  console.log(`Saved assets/${file.name} (${fileBuf.length} bytes - Clean)`);
}

// Automatically generate app.ico from the binary's extracted SVG favicon
const svgAsset = webAssets.find(f => f.name.endsWith(".svg"));
if (svgAsset) {
  const svgPath = path.join(assetsDir, svgAsset.name);
  console.log(`Generating application icon directly from extracted ${svgAsset.name}...`);
  try {
    const { Resvg } = await import("@resvg/resvg-js");
    const svgData = fs.readFileSync(svgPath);
    const sizes = [16, 24, 32, 48, 64, 128, 256];
    const pngList = sizes.map(size => {
      const resvg = new Resvg(svgData, { fitTo: { mode: "width", value: size } });
      return { size, buffer: resvg.render().asPng() };
    });

    const header = Buffer.alloc(6);
    header.writeUInt16LE(0, 0);
    header.writeUInt16LE(1, 2);
    header.writeUInt16LE(pngList.length, 4);

    let offset = 6 + 16 * pngList.length;
    const entries = [];
    for (const item of pngList) {
      const entry = Buffer.alloc(16);
      entry.writeUInt8(item.size >= 256 ? 0 : item.size, 0);
      entry.writeUInt8(item.size >= 256 ? 0 : item.size, 1);
      entry.writeUInt8(0, 2);
      entry.writeUInt8(0, 3);
      entry.writeUInt16LE(1, 4);
      entry.writeUInt16LE(32, 6);
      entry.writeUInt32LE(item.buffer.length, 8);
      entry.writeUInt32LE(offset, 12);
      entries.push(entry);
      offset += item.buffer.length;
    }

    const icoBuf = Buffer.concat([header, ...entries, ...pngList.map(i => i.buffer)]);
    const srcDir = path.join(rootDir, "src");
    fs.writeFileSync(path.join(srcDir, "app.ico"), icoBuf);
    fs.writeFileSync(path.join(srcDir, "app.rc"), '1 ICON "app.ico"\n');

    // Compile app.syso if windres is available
    try {
      execSync(`windres -O coff -o "${path.join(srcDir, "app.syso")}" "${path.join(srcDir, "app.rc")}"`, {
        cwd: srcDir,
        stdio: "ignore"
      });
      console.log("Successfully compiled src/app.syso with windres.");
    } catch {
      console.log("Note: windres not available in PATH; using existing src/app.syso if present.");
    }
    console.log("Updated application icon from upstream binary.");
  } catch (err) {
    console.warn("Could not generate icon from extracted SVG:", err.message);
  }
}

// Apply Windows polyfills & asset resolver to server.js
console.log("Applying Windows compatibility patches to server.js...");
const polyfill = `import * as __path from "node:path";
import * as __fsSync from "node:fs";
const __exeDir = typeof process !== "undefined" && process.execPath ? __path.dirname(process.execPath) : import.meta.dir;
function resolveAsset(name) {
  const inAssets = __path.join(__exeDir, "assets", name);
  if (__fsSync.existsSync(inAssets)) return inAssets;
  const inRoot = __path.join(__exeDir, name);
  if (__fsSync.existsSync(inRoot)) return inRoot;
  return __path.join(import.meta.dir, name);
}
`;

serverCode = serverCode.replace(/"(\/\$bunfs\/root\/)?([^"]+\.(svg|html|js|css))"/g, (match, prefix, p1) => {
  if (prefix) return `resolveAsset("${p1}")`;
  return match;
});
serverCode = serverCode.replace(/import\.meta\.dir \+ "\/([^"]+)"/g, (match, p1) => `resolveAsset("${p1}")`);

const patchedServerPath = path.join(tempDir, "server.js");
fs.writeFileSync(patchedServerPath, polyfill + "\n" + serverCode);
console.log("Saved patched server.js");

// Compile DrizzleGatewayServer.exe with Bun
console.log("Compiling DrizzleGatewayServer.exe with Bun...");
const serverExeOut = path.join(rootDir, "DrizzleGatewayServer.exe");
execSync(`bun build "${patchedServerPath}" --compile --outfile "${serverExeOut}"`, { stdio: "inherit" });

// Compile DrizzleGateway.exe with Go
console.log("Compiling DrizzleGateway.exe with Go...");
execSync(`go build -ldflags="-H windowsgui -s -w" -o "${path.join(rootDir, "DrizzleGateway.exe")}" ./src`, {
  cwd: rootDir,
  stdio: "inherit"
});

console.log(`\n Build complete for v${targetVersion}! Native Windows application ready.`);
