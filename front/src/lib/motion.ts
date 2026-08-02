import gsap from "gsap";
import { useGSAP } from "@gsap/react";
gsap.registerPlugin(useGSAP);
gsap.defaults({ ease: "power2.out", duration: 0.24 });
export { gsap, useGSAP };
