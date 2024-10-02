
// Import the CSS
import './assets/css/pico.purple.min.css';
import './assets/css/siimple-icons.css';
import './assets/css/main.css';

import logo from './assets/images/logo-universal.png';
import {LoadHosts} from "../wailsjs/go/app/App";


window.onload = function () {
    loadHosts();
};

// Setup the greet function
window.loadHosts = function () {


    try {
        LoadHosts()
            .then((result) => {

                let html = ""

                for (let i = 0; i < result.length; i++) {
                    let element = result[i];

                    html += `
                        <article class="host-article" onclick="editHost('${element.ID}')" data-id="${element.ID}" data-info='${JSON.stringify(element)}'>
                            <div class="grid">
                                <div>
                                    <span class="host-name">${element.Name}</span>
                                </div>
                                <div class="text-end">
                                    <button class="btn-action"><i class="si si-edit"></i></button>
                                </div>
                            </div>
                            
                        </article>
                    `
                }

                document.getElementById('host-container').innerHTML = html;
            })
            .catch((err) => {
                console.error(err);
            });
    } catch (err) {
        console.error(err);
    }
};

window.searchTerm = function (searchTerm){
    let hosts = document.querySelectorAll('.host-article');


    for (let i = 0; i < hosts.length; i++) {
        let host = hosts[i];
        let hostInfo = host.dataset.info;

        if (hostInfo.includes(searchTerm)) {
            host.style.display = 'block';

        } else {
            host.style.display = 'none';
        }

    }
}

window.showAside = function (){
    if(document.querySelector('aside').classList.contains('show')){
        hideAside();
        setTimeout(function (){
            document.querySelector('aside').classList.add('show');
        }, 200)
    }else{
        document.querySelector('aside').classList.add('show');
    }


    document.body.style.paddingRight = "340px";
}

window.hideAside = function (){
    document.querySelector('aside').classList.remove('show');
    document.body.style.paddingRight = "30px";
}

window.addHost = function (){
    document.querySelector('#host-form').reset();
    document.querySelector('#host-form').elements['id'].value = null;
    document.querySelector('#host-form').elements['id'].disabled = true;

    showAside();
}

window.editHost = function (hostId){

    let hostInfo = document.querySelector(`.host-article[data-id="${hostId}"]`).dataset.info;
    let host = JSON.parse(hostInfo);


    document.querySelector('#host-form').reset();
    document.querySelector('#host-form').elements['id'].value = host.ID;
    document.querySelector('#host-form').elements['id'].disabled = false;

    document.querySelector('#host-form').elements['name'].value = host.Name;
    document.querySelector('#host-form').elements['host'].value = host.Host;
    document.querySelector('#host-form').elements['port'].value = host.Port;
    document.querySelector('#host-form').elements['user'].value = host.User;
    document.querySelector('#host-form').elements['pass'].value = host.Pass;



    showAside();

}