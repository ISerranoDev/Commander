export namespace models {
	
	export class HostRecord {
	    ID: string;
	    Host: string;
	    Name: string;
	    User: string;
	    Pass: string;
	    Port: number;
	
	    static createFrom(source: any = {}) {
	        return new HostRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Host = source["Host"];
	        this.Name = source["Name"];
	        this.User = source["User"];
	        this.Pass = source["Pass"];
	        this.Port = source["Port"];
	    }
	}

}

