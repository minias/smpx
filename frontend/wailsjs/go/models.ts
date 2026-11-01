export namespace domain {
	
	export class PlaylistItem {
	    title: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new PlaylistItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.url = source["url"];
	    }
	}

}

export namespace service {
	
	export class PlayerState {
	    playlist: domain.PlaylistItem[];
	    currentIndex: number;
	    state: string;
	    loopMode: string;
	    volume: number;
	
	    static createFrom(source: any = {}) {
	        return new PlayerState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.playlist = this.convertValues(source["playlist"], domain.PlaylistItem);
	        this.currentIndex = source["currentIndex"];
	        this.state = source["state"];
	        this.loopMode = source["loopMode"];
	        this.volume = source["volume"];
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

}

export namespace update {
	
	export class Info {
	    version: string;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.url = source["url"];
	    }
	}

}

