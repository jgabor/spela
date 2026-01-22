export namespace gui {
	
	export class CPUInfo {
	    model: string;
	    cores: number;
	    averageFrequency: number;
	    governor: string;
	    smtEnabled: boolean;
	    utilizationPercent: number;
	    memoryUsedMegabytes: number;
	    memoryTotalMegabytes: number;
	
	    static createFrom(source: any = {}) {
	        return new CPUInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.model = source["model"];
	        this.cores = source["cores"];
	        this.averageFrequency = source["averageFrequency"];
	        this.governor = source["governor"];
	        this.smtEnabled = source["smtEnabled"];
	        this.utilizationPercent = source["utilizationPercent"];
	        this.memoryUsedMegabytes = source["memoryUsedMegabytes"];
	        this.memoryTotalMegabytes = source["memoryTotalMegabytes"];
	    }
	}
	export class ConfigInfo {
	    logLevel: string;
	    shaderCache: string;
	    checkUpdates: boolean;
	    showHints: boolean;
	    rescanOnStartup: boolean;
	    autoUpdateDLLs: boolean;
	    steamPath: string;
	    additionalLibraryPaths: string[];
	    dllCachePath: string;
	    backupPath: string;
	    dllManifestURL: string;
	    autoRefreshManifest: boolean;
	    manifestRefreshHours: number;
	    preferredDLLSource: string;
	    theme: string;
	    compactMode: boolean;
	    confirmDestructive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConfigInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.logLevel = source["logLevel"];
	        this.shaderCache = source["shaderCache"];
	        this.checkUpdates = source["checkUpdates"];
	        this.showHints = source["showHints"];
	        this.rescanOnStartup = source["rescanOnStartup"];
	        this.autoUpdateDLLs = source["autoUpdateDLLs"];
	        this.steamPath = source["steamPath"];
	        this.additionalLibraryPaths = source["additionalLibraryPaths"];
	        this.dllCachePath = source["dllCachePath"];
	        this.backupPath = source["backupPath"];
	        this.dllManifestURL = source["dllManifestURL"];
	        this.autoRefreshManifest = source["autoRefreshManifest"];
	        this.manifestRefreshHours = source["manifestRefreshHours"];
	        this.preferredDLLSource = source["preferredDLLSource"];
	        this.theme = source["theme"];
	        this.compactMode = source["compactMode"];
	        this.confirmDestructive = source["confirmDestructive"];
	    }
	}
	export class DLLInfo {
	    name: string;
	    path: string;
	    version: string;
	    dllType: string;
	
	    static createFrom(source: any = {}) {
	        return new DLLInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.version = source["version"];
	        this.dllType = source["dllType"];
	    }
	}
	export class DLLUpdateInfo {
	    name: string;
	    currentVersion: string;
	    latestVersion: string;
	    hasUpdate: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DLLUpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.hasUpdate = source["hasUpdate"];
	    }
	}
	export class GPUInfo {
	    name: string;
	    temperature: number;
	    powerDraw: number;
	    powerLimit: number;
	    utilization: number;
	    memoryUsed: number;
	    memoryTotal: number;
	    graphicsClock: number;
	    memoryClock: number;
	
	    static createFrom(source: any = {}) {
	        return new GPUInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.temperature = source["temperature"];
	        this.powerDraw = source["powerDraw"];
	        this.powerLimit = source["powerLimit"];
	        this.utilization = source["utilization"];
	        this.memoryUsed = source["memoryUsed"];
	        this.memoryTotal = source["memoryTotal"];
	        this.graphicsClock = source["graphicsClock"];
	        this.memoryClock = source["memoryClock"];
	    }
	}
	export class GameInfo {
	    appId: number;
	    name: string;
	    installDir: string;
	    prefixPath: string;
	    dlls: DLLInfo[];
	    hasProfile: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GameInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appId = source["appId"];
	        this.name = source["name"];
	        this.installDir = source["installDir"];
	        this.prefixPath = source["prefixPath"];
	        this.dlls = this.convertValues(source["dlls"], DLLInfo);
	        this.hasProfile = source["hasProfile"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProfileInfo {
	    srMode: string;
	    srPreset: string;
	    srOverride: boolean;
	    fgEnabled: boolean;
	    fgOverride: boolean;
	    multiFrame: number;
	    indicator: boolean;
	    shaderCache: boolean;
	    threadedOptimization: boolean;
	    powerMizer: string;
	    enableHdr: boolean;
	    enableWayland: boolean;
	    enableNgxUpdater: boolean;
	    backupOnLaunch: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.srMode = source["srMode"];
	        this.srPreset = source["srPreset"];
	        this.srOverride = source["srOverride"];
	        this.fgEnabled = source["fgEnabled"];
	        this.fgOverride = source["fgOverride"];
	        this.multiFrame = source["multiFrame"];
	        this.indicator = source["indicator"];
	        this.shaderCache = source["shaderCache"];
	        this.threadedOptimization = source["threadedOptimization"];
	        this.powerMizer = source["powerMizer"];
	        this.enableHdr = source["enableHdr"];
	        this.enableWayland = source["enableWayland"];
	        this.enableNgxUpdater = source["enableNgxUpdater"];
	        this.backupOnLaunch = source["backupOnLaunch"];
	    }
	}

}

