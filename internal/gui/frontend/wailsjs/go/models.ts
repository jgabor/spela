export namespace main {
	
	export class CPUInfo {
	    model: string;
	    cores: number;
	    averageFrequency: number;
	    governor: string;
	    smtEnabled: boolean;
	
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
	    }
	}
	export class DLLInfo {
	    name: string;
	    path: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new DLLInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.version = source["version"];
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
	    preset: string;
	    srMode: string;
	    srOverride: boolean;
	    fgEnabled: boolean;
	    enableHdr: boolean;
	    enableWayland: boolean;
	    enableNgxUpdater: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.preset = source["preset"];
	        this.srMode = source["srMode"];
	        this.srOverride = source["srOverride"];
	        this.fgEnabled = source["fgEnabled"];
	        this.enableHdr = source["enableHdr"];
	        this.enableWayland = source["enableWayland"];
	        this.enableNgxUpdater = source["enableNgxUpdater"];
	    }
	}

}

